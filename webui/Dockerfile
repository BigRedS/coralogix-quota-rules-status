# syntax=docker/dockerfile:1.7

# Build a static webui binary, then ship it on distroless/static.
#
# Build context is the repo root (the webui imports internal/... packages).
# From the repo root:
#   docker build -f webui/Dockerfile -t blocked-status-webui .
#
# The server never writes to disk, so it runs cleanly with --read-only:
#   docker run --rm \
#       --read-only \
#       --cap-drop=ALL \
#       --security-opt=no-new-privileges \
#       -p 8765:8765 \
#       blocked-status-webui

FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ENV CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH}
RUN go build -trimpath -ldflags "-s -w" -o /out/webui ./webui

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/webui /webui
EXPOSE 8765
USER nonroot:nonroot
ENTRYPOINT ["/webui"]
CMD ["-listen", ":8765"]
