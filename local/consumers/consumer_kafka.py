import os
import time

from kafka import KafkaConsumer  # pypi: kafka-python


if __name__ == "__main__":
    # Wait for kafka and certificate.stream
    time.sleep(30)
    # Set up the consumer
    consumer = KafkaConsumer(
        os.getenv("TOPIC_NAME"),
        bootstrap_servers=os.getenv("KAFKA_HOST"),
    )
    for message in consumer:
        # Only showing a snippet of the "value" data.
        # This is only a demo, but real apps would deserialize the bytes data into JSON
        # and then peform some sort of business logic on it...
        print(f"Consumer received message: {message.value[:100]}")
