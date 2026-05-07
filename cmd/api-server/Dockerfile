FROM golang:1.26.2-bookworm AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o conure_api cmd/api-server/main.go

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=builder /workspace/conure_api /app/conure_api
USER 65532:65532
