FROM golang:1.25-alpine AS builder
ARG CMD

WORKDIR /app

COPY . .

RUN apk update && \
    apk add ca-certificates && \
    go mod download && \
    go build -C $CMD -o $(pwd)/bin/rabbit .

FROM alpine

WORKDIR /app

COPY .env .env
COPY --from=builder /app/bin/rabbit rabbit

CMD [ "/app/rabbit" ]
