FROM golang:1.19-alpine3.16 as builder

COPY . /app
WORKDIR /app/

RUN go build -v -o conure_services_apps ./main.go

FROM alpine:3.16
COPY --from=builder /app/conure_services_apps /app/conure_services_apps
WORKDIR /app
