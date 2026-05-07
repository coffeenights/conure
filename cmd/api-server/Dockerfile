FROM golang:1.26.2-bookworm AS builder

COPY . /app
WORKDIR /app/

RUN go build -v -o conure_api cmd/api-server/main.go

FROM alpine:3.19
COPY --from=builder /app/conure_api /app/conure_api
WORKDIR /app
