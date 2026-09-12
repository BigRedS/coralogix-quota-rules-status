# syntax=docker/dockerfile:1.7

# Build a static webui binary, then ship it on distroless/static.
#
# Build context is the repo root (the webui imports internal/... packages).
# From the repo root:
#   docker build -f webui/Dockerfile -t quota-rules-status-webui .
#
# The server never writes to disk, so it runs cleanly with --read-only:
#   docker run --rm \
#       --read-only \
#       --cap-drop=ALL \
#       --security-opt=no-new-privileges \
#       -p 8765:8765 \
#       quota-rules-status-webui

FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ENV CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH}
RUN go build -trimpath -ldflags "-s -w" -o /out/webui ./webui

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/webui /webui
EXPOSE 8765
# Numeric UID:GID, not the "nonroot" name — Kubernetes' runAsNonRoot check
# reads the image config's User field literally and can't verify a named
# user is non-root, so it refuses to start the container otherwise.
USER 65532:65532
ENTRYPOINT ["/webui"]
CMD ["-listen", ":8765"]
