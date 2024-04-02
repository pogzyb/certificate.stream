kafka:
	docker compose -f docker-compose-kafka.yml up --build
firehose:
	docker compose -f docker-compose-firehose.yml up --build
s3:
	docker compose -f docker-compose-s3.yml up --build
down:
	docker compose -f docker-compose-firehose.yml -f docker-compose-kafka.yml -f docker-compose-s3.yml down