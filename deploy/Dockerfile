FROM golang:alpine3.19 as build

WORKDIR /certificate.stream
COPY ../certificate.stream .

RUN apk update && apk add git \
    && go mod tidy \
    && go build -o cs .

FROM alpine:3.19

COPY --from=build /certificate.stream/cs /usr/local/bin/cs
RUN chmod +x /usr/local/bin/cs
USER guest

ENTRYPOINT [ "cs" ]
CMD [ "--version" ]
