import os
import time

import boto3


if __name__ == "__main__":
    # Wait for localstack
    time.sleep(30)

    s3 = boto3.client("s3", endpoint_url=os.getenv("AWS_ENDPOINT_URL"))
    bucket_name = os.getenv("BUCKET_NAME")
    seen = []

    while True:
        # List s3://<bucket>/<prefix> every 30 seconds.
        # This is a demo only, but real apps could react to s3:put operations in near-real-time...
        time.sleep(30)
        print(f"Listing bucket: {bucket_name}")
        resp = s3.list_objects_v2(
            Bucket=bucket_name,
            Prefix=os.getenv("BUCKET_PREFIX"),
        )
        contents = resp.get("Contents", [])
        for obj in contents:
            key = obj.get("Key")
            if key not in seen:
                print(f"{bucket_name}--->{key}")
                seen.append(key)
