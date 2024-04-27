package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"certificate.stream/service/certificate/v1"
	"certificate.stream/service/sink"
	"certificate.stream/service/stream"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/thediveo/enumflag/v2"
)

func init() {
	streamCmd.Flags().IntVarP(
		&workersPerSink, "workersForSink", "w", 5, "Number of concurrent workers for the sink.")
	streamCmd.Flags().IntVarP(
		&workersPerStream, "workersPerStream", "c", 2, "Number of concurrent workers per log stream.")
	streamCmd.Flags().IntVarP(
		&maxBatchSize, "maxBatchSize", "b", 200, "Maximum number of logs included in each sink put operation.")
	streamCmd.Flags().BoolVarP(
		&startFromRoot, "startFromRoot", "r", false, "Start streaming from the log's root (index=0).")
	streamCmd.Flags().BoolVarP(
		&debug, "debug", "d", false, "Enable verbose debug logging.")

	streamCmd.Flags().VarP(
		enumflag.NewWithoutDefault(&logSink, "Sink", sink.SinkSourceIds, enumflag.EnumCaseInsensitive),
		"sink", "s",
		"Streaming sink where to write batches of logs (e.g. kafka).")
	_ = streamCmd.MarkFlagRequired("sink")

	streamCmd.Flags().VarP(
		enumflag.NewSlice(&logOperators, "Operator", stream.OperatorIds, enumflag.EnumCaseInsensitive),
		"operator", "o",
		"Comma separated list of Log Operator names (e.g. certly,digicert,google).")

	streamCmd.Flags().VarP(
		enumflag.NewSlice(&logStatuses, "LogStatus", stream.LogStatusIds, enumflag.EnumCaseInsensitive),
		"status", "f",
		"Comma separated list of Log Status  (e.g. usable,undefined).")

	rootCmd.AddCommand(streamCmd)
}

var (
	logOperators     []stream.Operator  = []stream.Operator{stream.ALL}
	logStatuses      []stream.LogStatus = []stream.LogStatus{stream.Usable}
	logSink          sink.SinkSource
	workersPerSink   int
	workersPerStream int
	maxBatchSize     int
	startFromRoot    bool
	debug            bool

	streamCmd = &cobra.Command{
		Use:   "stream",
		Short: "Stream the certificate transparency logs",
		Long:  `Stream TLS certificates directly from the CT Logs into the given sink for analysis.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Set log level
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
			if debug {
				zerolog.SetGlobalLevel(zerolog.DebugLevel)
			}
			// Prevent skewed values/arguments
			if workersPerStream > 20 || workersPerStream < 1 {
				log.Fatal().Msg(fmt.Sprintf("workersPerStream %d is not recommended.", workersPerStream))
			}
			if workersPerSink > 20 || workersPerSink < 1 {
				log.Fatal().Msg(fmt.Sprintf("workersPerSink %d is not recommended.", workersPerSink))
			}
			if maxBatchSize > 500 || maxBatchSize < 10 {
				// Firehose can't handle more than 500 (4KB/record 1MB total)
				log.Fatal().Msg(fmt.Sprintf("batchMaxSize %d is not recommended.", maxBatchSize))
			}
			startIndex := -1
			if startFromRoot {
				startIndex = 0
			}
			// TODO: Implement a starting index argument/pattern.
			// Enables inspection of past certificates in the log tree, not just current.
			// Start index is specific to the given log operator and stream.
			Stream(logOperators, logStatuses, logSink,
				startIndex, maxBatchSize, workersPerStream, workersPerSink, debug)
		},
	}
)

func Stream(
	logOps []stream.Operator,
	logSts []stream.LogStatus,
	sinkSrc sink.SinkSource,
	startIndex, mbatchSize, nStreamWorkers, nSinkWorkers int,
	debug bool,
) {
	// Handle termination
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	// Create a cancellation context
	ctx, cancel := context.WithCancel(context.Background())
	// Set up the log sink
	logSink, err := sink.GetSink(ctx, sinkSrc)
	if err != nil {
		log.Fatal().Msg(fmt.Sprintf("sink error: %v", err))
	} else {
		log.Info().Msg(fmt.Sprintf("Using %s as sink", logSink.String()))
	}
	// Initialize the operators
	logOperators, err := stream.GetOperatorsFromArg(logOps)
	if err != nil {
		log.Fatal().Msg(fmt.Sprintf("could not get operators: %v", err))
	}
	// Create a channel for communication between log operators and the sink
	// TODO: should this channel size be configurable or algorithmic based on ops,streams,workers etc?
	fromLogsToSink := make(chan *certificate.Batch, nSinkWorkers*2)
	// Control the concurrenct number of put operations to the sink
	sinkSemaphore := make(chan struct{}, nSinkWorkers)
	// Start streaming logs from each operator
	var wg sync.WaitGroup
	go func() {
		defer close(fromLogsToSink)
		for _, logOperator := range logOperators {
			// Initialize streams for each operator
			logOperator.InitStreams(logSts, mbatchSize, nStreamWorkers, startIndex, debug)
			// Run this operator's streams in a goroutine
			wg.Add(1)
			go func(logOp *stream.LogOperator) {
				defer wg.Done()
				logOp.RunStreams(ctx, fromLogsToSink)
			}(logOperator)
		}
		// Wait for operators to finish
		wg.Wait()
	}()
	// Run forever
	for {
		select {
		case <-sigs:
			cancel()
			log.Info().Msg("Received termination signal")
			log.Info().Msg("Putting remaining logs to sink before exiting")
			for batch := range fromLogsToSink {
				_ = logSink.Put(context.Background(), batch)
			}
			os.Exit(1)
		case batch := <-fromLogsToSink:
			if batch != nil {
				go func() {
					defer func() { <-sinkSemaphore }()
					sinkSemaphore <- struct{}{}
					err := logSink.Put(context.Background(), batch)
					if err != nil {
						log.Debug().Msg(fmt.Sprintf("could not put to sink: %v", err))
					}
				}()
			}
		}
	}
}
