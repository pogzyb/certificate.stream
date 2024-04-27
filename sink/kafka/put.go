package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"certificate.stream/service/certificate/v1"

	"github.com/rs/zerolog/log"
	"github.com/segmentio/kafka-go"
)

type SinkKafka struct {
	clientKafka *kafka.Writer
	endpointURL string
	topicName   string
	partition   int
}

func (sk *SinkKafka) String() string {
	return fmt.Sprintf("Kafka=%s-%d", sk.topicName, sk.partition)
}

// Initializes the Firehose sink. Pulls Kafka configuration from the environment
// variables: SINK_KAFKA_TOPIC_NAME, SINK_KAFKA_PARTITION, SINK_KAFKA_ENDPOINT_URL
func (sk *SinkKafka) Init(ctx context.Context) error {
	topic := os.Getenv("SINK_KAFKA_TOPIC_NAME")
	endpointURL := os.Getenv("SINK_KAFKA_ENDPOINT_URL")
	partitionStr := os.Getenv("SINK_KAFKA_PARTITION")
	partition, err := strconv.Atoi(partitionStr)
	if err != nil {
		return err
	}
	endpointURLs := strings.Split(endpointURL, ",")
	w := &kafka.Writer{
		Addr:                   kafka.TCP(endpointURLs...),
		Topic:                  topic,
		Balancer:               &kafka.RoundRobin{},
		AllowAutoTopicCreation: true,
	}
	sk.partition = partition
	sk.endpointURL = endpointURL
	sk.topicName = topic
	sk.clientKafka = w
	return nil
}

// Sends all certificate logs in this batch to the Kafka topic.
func (sk *SinkKafka) Put(ctx context.Context, batch *certificate.Batch) error {
	kfRecords := make([]kafka.Message, 0, len(batch.Logs))
	for _, logEntry := range batch.Logs {
		data, err := json.Marshal(logEntry)
		if err != nil {
			log.Error().Msg(fmt.Sprintf("could not marshal log entry: %v", err))
			continue
		}
		kfRecords = append(kfRecords, kafka.Message{Value: data})
	}
	// Put batch of records to kafka
	err := sk.clientKafka.WriteMessages(context.Background(), kfRecords...)
	if err == nil {
		log.Debug().Msg(
			fmt.Sprintf("%s:%s batch=[%d:%d] put to %s",
				batch.OperatorName, batch.LogSourceName, batch.Start, batch.End,
				sk.String()),
		)
	}
	return err
}
