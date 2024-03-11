package firehose

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"certificate.stream/service/pkg/certificate/v1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/firehose"
	"github.com/aws/aws-sdk-go-v2/service/firehose/types"
	"github.com/rs/zerolog/log"
)

type SinkFirehose struct {
	clientFirehose     *firehose.Client
	deliveryStreamName string
}

func (sf *SinkFirehose) String() string {
	return fmt.Sprintf("Firehose=%s", sf.deliveryStreamName)
}

// Initializes the Firehose sink. Pulls AWS credentials from the environment
// and the delivery stream name from SINK_FIREHOSE_DELIVERY_STREAM_NAME.
func (sf *SinkFirehose) Init(ctx context.Context) error {
	conf, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	if os.Getenv("AWS_ENDPOINT_URL") != "" {
		conf.BaseEndpoint = aws.String(os.Getenv("AWS_ENDPOINT_URL"))
	}
	sf.clientFirehose = firehose.NewFromConfig(conf)
	sf.deliveryStreamName = os.Getenv("SINK_FIREHOSE_DELIVERY_STREAM_NAME")
	if sf.deliveryStreamName == "" {
		return fmt.Errorf("missing environment variable: SINK_FIREHOSE_DELIVERY_STREAM_NAME")
	}
	_, err = sf.clientFirehose.DescribeDeliveryStream(
		ctx,
		&firehose.DescribeDeliveryStreamInput{
			DeliveryStreamName: aws.String(sf.deliveryStreamName),
		},
	)
	return err
}

// Sends all certificate logs in this batch to the Firehose delivery stream
func (sf *SinkFirehose) Put(ctx context.Context, batch *certificate.Batch) error {
	fhRecords := make([]types.Record, 0, len(batch.Logs))
	for _, logEntry := range batch.Logs {
		data, err := json.Marshal(logEntry)
		if err != nil {
			log.Error().Msg(fmt.Sprintf("could not marshal log entry: %v", err))
			continue
		}
		// https://stackoverflow.com/questions/48226472/kinesis-firehose-putting-json-objects-in-s3-without-seperator-comma
		// AWS firehose just smashes all records together into a base64 encoded blob, so
		// we add a comma between records to enable downstream json decoding.
		data = append(data, []byte(",")[:]...)
		fhRecords = append(fhRecords, types.Record{Data: data})
	}
	// Put batch of records to firehose
	batchInput := &firehose.PutRecordBatchInput{
		DeliveryStreamName: &sf.deliveryStreamName,
		Records:            fhRecords,
	}
	_, err := sf.clientFirehose.PutRecordBatch(ctx, batchInput)
	if err == nil {
		log.Debug().Msg(
			fmt.Sprintf("%s:%s batch=[%d:%d] put to %s",
				batch.OperatorName, batch.LogSourceName, batch.Start, batch.End, sf.String()))
	}
	return err
}
