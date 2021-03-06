FROM golang:1.14 as builder

ARG GOOS=linux
ARG GOARCH=amd64

WORKDIR "/code"
ADD . "/code"
RUN make BINARY=rabbitmq-mv GOOS=${GOOS} GOARCH=${GOARCH} os.build

FROM alpine:3.11
COPY --from=builder /code/rabbitmq-mv /rabbitmq-mv
ENTRYPOINT ["/rabbitmq-mv"]
