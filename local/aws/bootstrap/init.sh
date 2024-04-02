#!/usr/bin/env bash

apt install -y jq gettext-base

BUCKET_NAME=ctlogs
DELIVERY_STREAM_NAME=ctlogs
KINESIS_STREAM_NAME=ctlogs
REGION=us-east-1

awslocal s3api create-bucket \
   --bucket ${BUCKET_NAME} \
   --region ${REGION}

if [[ $SERVICES == *"firehose"* ]]; then
    awslocal kinesis create-stream \
        --stream-name ${KINESIS_STREAM_NAME} \
        --shard-count 1 \
        --region ${REGION}

    export KIN_STREAM_ARN=$(awslocal kinesis describe-stream \
        --stream-name ${KINESIS_STREAM_NAME} \
        --region ${REGION} | jq -r '.StreamDescription.StreamARN')
        echo "$KIN_STREAM_ARN"

    awslocal firehose create-delivery-stream \
        --delivery-stream-type KinesisStreamAsSource \
        --delivery-stream-name ${DELIVERY_STREAM_NAME} \
        --kinesis-stream-source-configuration KinesisStreamARN="$KIN_STREAM_ARN",RoleARN=arn:aws:iam:::firehoseRole \
        --s3-destination-configuration file:///tmp/resources/firehose-s3.json \
        --region ${REGION}
fi
