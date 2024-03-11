FROM golang:alpine3.19 as build

WORKDIR /certificate.stream
COPY . .

RUN apk update && apk add git \
    && go mod tidy \
    && go build -o certificate .

FROM alpine:3.19

ENV VERSION=0.1.dev0

ARG CREATED
ARG REVISION

LABEL org.opencontainers.image.title="Certificate Stream"
LABEL org.opencontainers.image.description="Certificate Transparency Log monitoring for everybody."
LABEL org.opencontainers.image.version=VERSION
LABEL org.opencontainers.image.authors="pogzyb@umich.edu"
LABEL org.opencontainers.image.url="https://github.com/pogzyb/certificate.stream"
LABEL org.opencontainers.image.source="https://github.com/pogzyb/certificate.stream/certificate.stream"
LABEL org.opencontainers.image.documentation="https://github.com/pogzyb/certificate.stream"
LABEL org.opencontainers.image.created=CREATED
LABEL org.opencontainers.image.revision=REVISION
LABEL org.opencontainers.image.licenses="MIT License"

COPY --from=build /certificate.stream/certificate /usr/local/bin/certificate
RUN chmod u+x /usr/local/bin/certificate

USER guest

ENTRYPOINT [ "certificate" ]
CMD [ "--help" ]
