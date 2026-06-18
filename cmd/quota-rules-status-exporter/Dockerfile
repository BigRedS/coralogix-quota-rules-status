# syntax=docker/dockerfile:1.7

# Container image for the exporter, for one-shot/cron use (e.g. a k8s CronJob).
#
# Build context is the repo root:
#   docker build -f cmd/quota-rules-status-exporter/Dockerfile -t quota-rules-status-exporter .
#
# Run once (reads + emits, then exits):
#   docker run --rm \
#       -e CX_REGION=eu2 -e CX_API_KEY=... \
#       -e CX_SEND_YOUR_DATA_KEY=... -e CX_TEAM=otel-demo \
#       quota-rules-status-exporter
#
# For AWS Lambda, don't use this image — build the bootstrap zip instead
# (see cmd/quota-rules-status-exporter/README.md).

FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=arm64
ENV CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH}
RUN go build -trimpath -ldflags "-s -w" -o /out/exporter ./cmd/quota-rules-status-exporter

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/exporter /exporter
USER nonroot:nonroot
ENTRYPOINT ["/exporter"]
