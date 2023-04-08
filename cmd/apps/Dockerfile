FROM golang:1.19-alpine3.16 as builder

COPY . /app
WORKDIR /app/

RUN go build -v -o services_apps cmd/apps/main.go

FROM alpine:3.16
COPY --from=builder /app/services_apps /app/services_apps
WORKDIR /app
