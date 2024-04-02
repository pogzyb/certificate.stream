package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"certificate.stream/service/pkg/certificate/v1"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type SinkS3 struct {
	clientS3            *s3.Client
	bucketName          string
	bucketPrefix        string
	useDatePartitioning bool
}

func (ss *SinkS3) String() string {
	return fmt.Sprintf("S3://%s/%s", ss.bucketName, ss.bucketPrefix)
}

// Initializes the Firehose sink. Pulls AWS credentials from the environment
// and the bucket information from SINK_S3_BUCKET_NAME and SINK_S3_BUCKET_PREFIX.
func (ss *SinkS3) Init(ctx context.Context) error {
	conf, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	ss.clientS3 = s3.NewFromConfig(conf, func(o *s3.Options) {
		if os.Getenv("AWS_ENDPOINT_URL") != "" {
			o.BaseEndpoint = aws.String(os.Getenv("AWS_ENDPOINT_URL"))
			o.UsePathStyle = true
		}
	})
	ss.bucketName = os.Getenv("SINK_S3_BUCKET_NAME")
	if ss.bucketName == "" {
		return fmt.Errorf("missing environment variable: SINK_S3_BUCKET_NAME")
	}
	ss.bucketPrefix = os.Getenv("SINK_S3_BUCKET_PREFIX")
	if ss.bucketPrefix == "" {
		return fmt.Errorf("missing environment variable: SINK_S3_BUCKET_PREFIX")
	}
	usePartitioning := os.Getenv("SINK_S3_USE_DATE_PARTITIONING")
	if usePartitioning != "" {
		val, err := strconv.ParseBool(usePartitioning)
		if err == nil {
			ss.useDatePartitioning = val
			log.Debug().Msg(fmt.Sprintf("Sink %s useDatePartitioning=%t", ss.String(), ss.useDatePartitioning))
		} else {
			return err
		}
	}
	return nil
}

// Sends all certificate logs in this batch to the s3 bucket.
func (ss *SinkS3) Put(ctx context.Context, batch *certificate.Batch) error {
	// unique filename
	now := time.Now()
	filename := fmt.Sprintf("%d_%s.json", now.UnixMicro(), uuid.NewString())
	var key string
	if ss.useDatePartitioning {
		partitionPath := fmt.Sprintf("year=%d/month=%02d/day=%02d",
			now.Year(), int(now.Month()), now.Day())
		key = filepath.Join(ss.bucketPrefix, partitionPath, filename)
	} else {
		key = filepath.Join(ss.bucketPrefix, filename)
	}
	bodyBytes, err := json.Marshal(batch.Logs)
	if err != nil {
		return err
	}
	bodyReader := bytes.NewReader(bodyBytes)
	_, err = ss.clientS3.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(ss.bucketName),
		Key:    aws.String(key),
		Body:   bodyReader,
	})
	if err == nil {
		log.Debug().Msg(
			fmt.Sprintf("%s:%s batch=[%d:%d] put to %s",
				batch.OperatorName, batch.LogSourceName, batch.Start, batch.End,
				ss.String()),
		)
	}
	return err
}
