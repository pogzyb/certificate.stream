package sink

import (
	"context"
	"fmt"

	"certificate.stream/service/certificate/v1"
	"certificate.stream/service/sink/file"
	"certificate.stream/service/sink/firehose"
	"certificate.stream/service/sink/kafka"
	"certificate.stream/service/sink/s3"
	"github.com/thediveo/enumflag/v2"
)

type SinkSource enumflag.Flag

const (
	NoDefault SinkSource = iota // optional; must be the zero value.
	Firehose
	Kafka
	File
	S3
)

var SinkSourceIds = map[SinkSource][]string{
	Firehose: {"firehose"},
	Kafka:    {"kafka"},
	File:     {"file"},
	S3:       {"s3"},
}

type Sink interface {
	Init(ctx context.Context) error
	Put(ctx context.Context, batch *certificate.Batch) error
	String() string
}

func GetSink(ctx context.Context, ss SinkSource) (Sink, error) {
	switch ss {
	case S3:
		sink := &s3.SinkS3{}
		err := sink.Init(ctx)
		return sink, err

	case File:
		sink := &file.SinkFile{}
		err := sink.Init(ctx)
		return sink, err

	case Firehose:
		sink := &firehose.SinkFirehose{}
		err := sink.Init(ctx)
		return sink, err

	case Kafka:
		sink := &kafka.SinkKafka{}
		err := sink.Init(ctx)
		return sink, err

	default:
		return nil, fmt.Errorf("no such sink source: %v", ss)
	}
}
