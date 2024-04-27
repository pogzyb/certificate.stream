package stream

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"certificate.stream/service/certificate/v1"
	"github.com/google/certificate-transparency-go/loglist3"
	"github.com/rs/zerolog/log"
	"github.com/thediveo/enumflag/v2"
)

type Operator enumflag.Flag

const (
	ALL Operator = iota
	Google
	Cloudflare
	DigiCert
	Certly
	Izenpe
	WoSign
	Venafi
	CNNIC
	StartCom
	Sectigo
	LetsEncrypt
	TrustAsia
	WangShengnan
	GDCA
	BeijingPuChuangSiDaTechnologyLtd
	NORDUnet
	SHECA
	Akamai
	MattPalmer
	UpInTheAirConsulting
	Qihoo360
)

var (
	OperatorIds = map[Operator][]string{
		ALL:                              {"all"},
		Google:                           {"google"},
		Cloudflare:                       {"cloudflare"},
		DigiCert:                         {"digicert"},
		Certly:                           {"certly"},
		Izenpe:                           {"izenpe"},
		WoSign:                           {"wosign"},
		Venafi:                           {"venafi"},
		CNNIC:                            {"cnnic"},
		StartCom:                         {"startcom"},
		Sectigo:                          {"sectigo"},
		LetsEncrypt:                      {"letsencrypt"},
		TrustAsia:                        {"trustasia"},
		WangShengnan:                     {"wangshengnan"},
		GDCA:                             {"gdca"},
		BeijingPuChuangSiDaTechnologyLtd: {"bptl"},
		NORDUnet:                         {"nordunet"},
		SHECA:                            {"sheca"},
		Akamai:                           {"akamai"},
		MattPalmer:                       {"mattpalmer"},
		UpInTheAirConsulting:             {"uitac"},
		Qihoo360:                         {"qihoo360"},
	}

	operatorLogListNames = map[string]string{
		"google":       "Google",
		"cloudflare":   "Cloudflare",
		"digicert":     "DigiCert",
		"certly":       "Certly",
		"izenpe":       "Izenpe",
		"wosign":       "WoSign",
		"venafi":       "Venafi",
		"cnnic":        "CNNIC",
		"startcom":     "StartCom",
		"sectigo":      "Sectigo",
		"letsencrypt":  "Let's Encrypt",
		"trustasia":    "TrustAsia",
		"wangshengnan": "Wang Shengnan",
		"gdca":         "GDCA",
		"bptl":         "Beijing PuChuangSiDa Technology Ltd.",
		"nordunet":     "NORDUnet",
		"sheca":        "SHECA",
		"akamai":       "Akamai",
		"mattpalmer":   "Matt Palmer",
		"uitac":        "Up In The Air Consulting",
		"qihoo360":     "Qihoo 360",
	}
)

func getOperatorFromLogList(op Operator) (*loglist3.Operator, error) {
	opStrings, ok := OperatorIds[op]
	if !ok {
		return nil, fmt.Errorf("invalid operator: %v", op)
	}
	opName := opStrings[0]
	opString := operatorLogListNames[opName]
	ll, err := GetLogList()
	if err != nil {
		return nil, err
	}
	for _, l := range ll.Operators {
		if l.Name == opString {
			return l, nil
		}
	}
	return nil, fmt.Errorf("could not get operator: %s", opName)
}

func GetOperatorsFromArg(ops []Operator) ([]*LogOperator, error) {
	var operators []*LogOperator
	if ops[0] == ALL {
		for op := range OperatorIds {
			if op == ALL {
				continue
			}
			llOp, err := getOperatorFromLogList(op)
			if err != nil {
				return nil, err
			}
			operator := &LogOperator{
				Name:       llOp.Name,
				Operator:   llOp,
				LogStreams: nil,
			}
			operators = append(operators, operator)
		}
	} else {
		for _, op := range ops {
			if op == ALL {
				// edge case for value: "google,ALL,digicert"
				continue
			}
			llOp, err := getOperatorFromLogList(op)
			if err != nil {
				return nil, err
			}
			operator := &LogOperator{
				Name:       llOp.Name,
				Operator:   llOp,
				LogStreams: nil,
			}
			operators = append(operators, operator)
		}
	}

	return operators, nil
}

type LogOperator struct {
	Name       string
	Operator   *loglist3.Operator
	LogStreams []*LogStream
}

// Populates `[]*LogStreams` for this log operator. Log operators can have
// multiple log streams (HTTPS endpoints) where the operator puts certificate logs.
// This function will initialize all the non-retired and non-rejected log streams
// for the given operator.
func (lo *LogOperator) InitStreams(statuses []LogStatus, bSize, nWorkers, skipTo int, verbose bool) {
	var useStatuses []loglist3.LogStatus
	for _, st := range statuses {
		llSt := logStatusToLLStatusMap[st]
		useStatuses = append(useStatuses, llSt)
	}
	var logStreams []*LogStream
	for _, ll := range lo.Operator.Logs {
		status := ll.State.LogStatus()
		if slices.Contains(useStatuses, status) {
			ls, err := InitLogStream(ll.URL, lo.Name, bSize, nWorkers, skipTo, verbose)
			if err != nil {
				log.Error().Msg(fmt.Sprintf("error log-source=%s: %v", ll.URL, err))
				continue
			}
			logStreams = append(logStreams, ls)
		}
	}
	lo.LogStreams = logStreams
}

// Streams certificate logs from this log operator. This function is run in
// a goroutine and its execution can be stopped or cancelled via the context parameter.
// This function will close the `toSink` channel when its execution is finished.
func (lo *LogOperator) RunStreams(ctx context.Context, toSink chan *certificate.Batch) {
	// Check to make sure there are streams.
	if len(lo.LogStreams) == 0 {
		log.Info().Msg(fmt.Sprintf("Operator=[%s] [0] streams", lo.Name))
		return
	}
	// Each LogStream for this LogOperator gets its own channel on which
	// it sends batches of certificates and closes if the context is cancelled.
	// These channels are aggregated and sent to the `outgoing` channel.
	streams := make([]chan *certificate.Batch, len(lo.LogStreams))
	// Start each LogStream
	var wg sync.WaitGroup
	for _, ls := range lo.LogStreams {
		wg.Add(1)
		stream := make(chan *certificate.Batch, 25)
		streams = append(streams, stream)
		// Each stream receives the channel on which to send back the
		// certificates it finds as well as a context which is used
		// to control the lifetime of the goroutine.
		go func(logSt *LogStream, crtSt chan *certificate.Batch) {
			defer wg.Done()
			logSt.Stream(ctx, crtSt)
		}(ls, stream)
	}
	log.Info().Msg(fmt.Sprintf("Operator=[%s] %d streams", lo.Name, len(streams)))
	// Receive from each stream's channel in a goroutine, sending batches
	// to the outbound sink's channel. Borrowed this pattern from:
	// https://stackoverflow.com/questions/19992334/how-to-listen-to-n-channels-dynamic-select-statement
	for _, stream := range streams {
		go func(ch chan *certificate.Batch) {
			for batch := range ch {
				toSink <- batch
			}
		}(stream)
	}
	// Wait forever
	wg.Wait()
}
