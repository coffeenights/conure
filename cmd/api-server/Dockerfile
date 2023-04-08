FROM golang:1.19-alpine3.16 as builder

COPY . /app
WORKDIR /app/

RUN go build -v -o conure_api cmd/api-server/main.go

FROM alpine:3.16
COPY --from=builder /app/conure_api /app/conure_api
WORKDIR /app
