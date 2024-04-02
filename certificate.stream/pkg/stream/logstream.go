package stream

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	"certificate.stream/service/pkg/certificate/v1"
	"github.com/cenkalti/backoff/v4"
	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/jsonclient"
	"github.com/google/certificate-transparency-go/x509"
	"github.com/rs/zerolog/log"
)

var userAgent = "ct-go-scanlog/1.0"

type BatchIndex struct {
	Start int64
	End   int64
}

type LogStream struct {
	LogClient     *client.LogClient
	OperatorName  string
	LogSourceName string
	STH           *ct.SignedTreeHead
	BatchSize     int
	NWorkers      int
	IndexStart    int64
	IndexEnd      int64
	Verbose       bool
}

func (ls *LogStream) String() string {
	return fmt.Sprintf("%s:%s", ls.OperatorName, ls.LogSourceName)
}

func InitLogStream(operatorURI, operatorName string, batchSize, nWorkers, skipToIndex int, verbose bool) (*LogStream, error) {
	logClient, err := client.New(
		operatorURI,
		&http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSHandshakeTimeout:   30 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
				MaxIdleConnsPerHost:   10,
				DisableKeepAlives:     false,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
		jsonclient.Options{UserAgent: userAgent},
	)
	if err != nil {
		log.Error().Msg(fmt.Sprintf("could not get CT client: %v", err))
		return nil, err
	}
	ls := &LogStream{
		LogClient:     logClient,
		OperatorName:  operatorName,
		LogSourceName: certificate.GetSourceName(logClient.BaseURI()),
		BatchSize:     batchSize,
		IndexStart:    int64(skipToIndex),
		Verbose:       verbose,
		NWorkers:      nWorkers,
	}
	return ls, nil
}

func (ls *LogStream) Stream(ctx context.Context, toAgg chan<- *certificate.Batch) {
	defer close(toAgg)
	// Steps 1 and 2
	err := ls.updateSTH(ctx)
	if err != nil {
		log.Error().Msg(fmt.Sprintf("could not get STH: %v", err))
		return
	}
	// Step 3 in batches
	batches := ls.fetchBatches(ctx)
	// Multiple workers fetch certificates from the CT Log endpoints
	// and send batches to the `toAgg` channel.
	var wg sync.WaitGroup
	for n := 0; n < ls.NWorkers; n++ {
		wg.Add(1)
		go func(workerN int) {
			defer wg.Done()
			ls.fetchCertificates(ctx, toAgg, batches)
		}(n)
	}
	// Wait for workers to finish
	wg.Wait()
	log.Debug().Msg(fmt.Sprintf("%s workers have stopped", ls.String()))
}

// Updates the signed tree head.
// In other words, this function will fetch the current "root" of the merkle tree,
// and set the new start and end indices that need to be fetched from the log.
func (ls *LogStream) updateSTH(ctx context.Context) error {
	// 1.  Fetch the current STH (Section 4.3).
	var head *ct.SignedTreeHead
	// Function to retrieve the signed tree head
	attempts := 0
	fn := func() error {
		if ctx.Err() != nil {
			// Context was cancelled, so return nil so that the backoff stops.
			// TODO: there should be a better way of handling this.
			return nil
		}
		var err error
		head, err = ls.LogClient.GetSTH(ctx)
		if err != nil {
			attempts += 1
			log.Debug().Msg(fmt.Sprintf("%s [attempts=%d] could not get STH: %v",
				ls.String(),
				attempts,
				err,
			))
			return err
		}
		if int64(head.TreeSize) <= ls.IndexEnd {
			err = fmt.Errorf("STH has not changed")
			attempts += 1
			log.Debug().Msg(fmt.Sprintf("%s [attempts=%d]: %v",
				ls.String(),
				attempts,
				err,
			))
			return err
		}
		ls.STH = head
		return nil
	}
	// Use the function above to fetch STH.
	// Exponential backoff is used to retry the function in case
	// the STH hasn't been updated by the log source operator
	bo := randomBackoff(ctx)
	if err := backoff.Retry(fn, bo); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// 2.  Verify the STH signature.
	if err := ls.LogClient.VerifySTHSignature(*head); err != nil {
		log.Debug().Msg(fmt.Sprintf("could not verify STH: %v", err))
		return err
	}
	// update start and end indices
	if ls.IndexStart == -1 {
		// start from head
		ls.IndexStart = int64(head.TreeSize)
	} else if ls.IndexStart != 0 && ls.IndexStart > 0 {
		// start from custom index
		ls.IndexStart = ls.IndexEnd
	} else {
		// handle bad values
		ls.IndexStart = 0
	}
	ls.IndexEnd = int64(head.TreeSize)
	log.Debug().Msg(
		fmt.Sprintf("%s STH updated: start=%d end=%d",
			ls.String(),
			ls.IndexStart,
			ls.IndexEnd,
		))
	return nil
}

func randomBackoff(ctx context.Context) backoff.BackOffContext {
	b := backoff.NewExponentialBackOff()
	b.MaxInterval = time.Minute * 10     // wait a maximum of 10 minutes in 1 interval
	b.MaxElapsedTime = time.Hour * 5     // wait a maximum of 5 hours total
	b.InitialInterval = time.Second * 60 // wait 60 seconds the first retry
	b.Multiplier = 1.5
	b.RandomizationFactor = 0.75
	boCtx := backoff.WithContext(b, ctx)
	return boCtx
}

func (ls *LogStream) fetchBatches(ctx context.Context) <-chan BatchIndex {
	batches := make(chan BatchIndex)
	go func() {
		// Close batch channel when function exits
		defer close(batches)

		start := ls.IndexStart
		end := ls.IndexEnd
		// Create `batchSize` batches within the interval "start">"end" indices
		// and put them onto the batches channel.
		for {
			if ctx.Err() != nil {
				return
			}
			if start >= end {
				// At this point we are finished reading the entries in this log,
				// so we need to update the STH to check for more logs.
				err := ls.updateSTH(ctx)
				if err != nil {
					log.Error().Msg(
						fmt.Sprintf("%s could not update STH: %v", ls.String(), err))
					return
				}
				log.Debug().Msg(
					fmt.Sprintf("%s STH updated: start=%d end=%d", ls.String(), start, end))
				end = ls.IndexEnd
			}
			// Generate a batch and put it on the channel
			batchEnd := start + int64(math.Min(float64(end-start), float64((ls.BatchSize))))
			batch := BatchIndex{Start: start, End: batchEnd - 1}
			select {
			case batches <- batch:
			case <-ctx.Done():
				log.Debug().Msg(fmt.Sprintf("%s will stop sending batches", ls.String()))
				return
			}
			log.Debug().Msg(
				fmt.Sprintf("%s batch created: start=%d end=%d",
					ls.String(),
					start,
					batchEnd,
				))
			start = batchEnd
		}
	}()
	return batches
}

func (ls *LogStream) fetchCertificates(ctx context.Context, toAgg chan<- *certificate.Batch, batches <-chan BatchIndex) {
	// Receive incoming batch indices
	for batch := range batches {
		for batch.Start <= batch.End {
			// Define an entries response
			var entriesResp *ct.GetEntriesResponse
			// Create function to get cert entries from the log client
			fn := func() error {
				if ctx.Err() != nil {
					// Context was cancelled, so return nil so that the backoff stops.
					// TODO: there should be a better way of handling this.
					return nil
				}
				var err error
				entriesResp, err = ls.LogClient.GetRawEntries(ctx, batch.Start, batch.End)
				if err != nil {
					log.Error().Msg(
						fmt.Sprintf("%s could not GetRawEntries: %v", ls.String(), err))
					return err
				}
				return nil
			}
			// Try to fetch log entries; retry if there is an error.
			bo := randomBackoff(ctx)
			err := backoff.Retry(fn, bo)
			if err != nil {
				log.Debug().Msg(
					fmt.Sprintf("%s backoff for GetRawEntries failed: %v", ls.String(), err))
				return
			}
			if ctx.Err() != nil {
				// Context was cancelled, so stop.
				return
			}
			// Parse log entries
			certs := make([]certificate.Log, 0, len(entriesResp.Entries))
			for i, entry := range entriesResp.Entries {
				index := batch.Start + int64(i)
				logEntry, err := ct.LogEntryFromLeaf(index, &entry)
				if x509.IsFatal(err) {
					log.Error().Msg(
						fmt.Sprintf("%s could not parse x509 leaf cert: %v", ls.String(), err))
					continue
				}
				// Do custom parsing of LogEntry to CertPayload
				if cert := ls.newCertPayloadFromLogEntry(logEntry); cert != nil {
					certs = append(certs, *cert)
				}
			}
			logBatch := &certificate.Batch{
				OperatorName:  ls.OperatorName,
				LogSourceName: ls.LogSourceName,
				Start:         batch.Start,
				End:           batch.End,
				Logs:          certs,
			}
			toAgg <- logBatch
			batch.Start += int64(len(entriesResp.Entries))
		}
	}
}

func (ls *LogStream) newCertPayloadFromLogEntry(entry *ct.LogEntry) *certificate.Log {
	// https://github.com/CaliDog/certstream-python/issues/13
	// p.s. Depending on your use case, I'd recommend against excluding pre-certificates
	// from your search - not all CAs log the final certificate (I believe DigiCert, GoDaddy, and Amazon don't),
	// so you'll miss some final certificates unless a third party finds and submits them.
	if entry.X509Cert != nil {
		// handle cert data
		payload := &certificate.Log{
			EntryType: "X509Cert",
			Body: certificate.X509LogEntry{
				Index:    entry.Index,
				Date:     entry.X509Cert.NotBefore.Format("2006-01-02"),
				IssuedAt: entry.X509Cert.NotBefore,
				Source: certificate.LogSource{
					URL:  ls.LogClient.BaseURI(),
					Name: ls.OperatorName,
				},
				Cert: *certificate.GetInfo(entry.X509Cert),
			},
		}
		// add issue and root certs
		for _, rawASN1 := range entry.Chain {
			cert, err := x509.ParseCertificate(rawASN1.Data)
			if err != nil {
				log.Error().Msg(
					fmt.Sprintf("could not parse certificate from ASN1 data: %v", err))
				continue
			}
			certInfo := certificate.GetInfo(cert)
			payload.Body.Chain = append(payload.Body.Chain, *certInfo)
		}
		// return the payload
		return payload
	} else if entry.Precert != nil {
		// handle pre-cert data
		payload := &certificate.Log{
			EntryType: "PreCert",
			Body: certificate.X509LogEntry{
				Index:    entry.Index,
				Date:     entry.Precert.TBSCertificate.NotBefore.Format("2006-01-02"),
				IssuedAt: entry.Precert.TBSCertificate.NotBefore,
				Source: certificate.LogSource{
					URL:  ls.LogClient.BaseURI(),
					Name: ls.OperatorName,
				},
				Cert: *certificate.GetInfo(entry.Precert.TBSCertificate),
			},
		}
		// add issue and root certs
		for _, rawASN1 := range entry.Chain {
			cert, err := x509.ParseCertificate(rawASN1.Data)
			if err != nil {
				log.Error().Msg(
					fmt.Sprintf("could not parse certificate from ASN1 data: %v", err))
				continue
			}
			certInfo := certificate.GetInfo(cert)
			payload.Body.Chain = append(payload.Body.Chain, *certInfo)
		}
		// return the payload
		return payload
	}
	return nil
}
