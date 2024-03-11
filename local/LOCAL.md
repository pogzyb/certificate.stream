### Try it out locally

| Command      | Description |
| ---------------------- | ---------------------- |
| `make kafka`      | Starts Kafka (zookeeper + 1 node), an example python consumer-app, and certificate.stream (Using the google operator only). The example consumer app will print messages as it receives them from Kafka. Note there is delay baked into the consumer app for simplicity, so messages are not consumed until ~30 seconds after start up.      |
| `make firehose`   | Starts Firehose (delivery stream + S3 bucket), a python container that lists the S3 bucket every 60s, and certificate.stream (Using the google operator only). Note there is delay baked into the consumer app for simplicity, so messages are not consumed until ~30 seconds after start up.             |
| `make down`   | Runs `docker-compose down` for clean-up.        |
