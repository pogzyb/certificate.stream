FROM python:3.11-alpine

RUN apk update && pip install boto3==1.34.59 kafka-python==2.0.2 

WORKDIR /consumer
COPY . .

ENTRYPOINT [ "python" ]
CMD [ "--version" ]