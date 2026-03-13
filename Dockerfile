# syntax=docker/dockerfile:1.6

############################
# Builder
############################
FROM golang:1.25.1-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /src/ra-openlan-nw-topology

# Cache deps
COPY ra-openlan-nw-topology/go.mod ra-openlan-nw-topology/go.sum ./
# Local module replacements (match go.mod replace paths)
COPY ra-common-mods/buildinfo /src/ra-common-mods/buildinfo
COPY ra-common-mods/kafka /src/ra-common-mods/kafka
COPY ra-common-mods/logger /src/ra-common-mods/logger
COPY ow-common-mods/service-discovery /src/ow-common-mods/service-discovery
COPY ow-common-mods/system-routes /src/ow-common-mods/system-routes
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy source
COPY ra-openlan-nw-topology/. .

# Build
ARG MAIN=./cmd            # because you have cmd/main.go
ARG APP_NAME=network-topology-service
ARG VERSION
ARG BUILD_TIMESTAMP
ARG COMMIT_HASH
ARG DEPLOYMENT_ENV=dev
ENV CGO_ENABLED=0 GOOS=linux GOFLAGS=-buildvcs=false

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go test ./...

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    mkdir -p /out && \
    VERSION_VALUE="${VERSION:-$(git describe --tags 2>/dev/null || echo -n '')}" && \
    BUILD_TIMESTAMP_VALUE="${BUILD_TIMESTAMP:-$(date -u +%s)}" && \
    COMMIT_HASH_VALUE="${COMMIT_HASH:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}" && \
    go build -trimpath \
    -ldflags="-s -w \
    -X github.com/routerarchitects/ra-common-mods/build-info.version=${VERSION_VALUE} \
    -X github.com/routerarchitects/ra-common-mods/build-info.buildTimestamp=${BUILD_TIMESTAMP_VALUE} \
    -X github.com/routerarchitects/ra-common-mods/build-info.commitHash=${COMMIT_HASH_VALUE} \
    -X github.com/routerarchitects/ra-common-mods/build-info.Environment=${DEPLOYMENT_ENV}" \
    -o "/out/${APP_NAME}" "${MAIN}"

############################
# Runtime Stage
############################
FROM alpine:3.20

RUN apk add --no-cache ca-certificates bash curl

WORKDIR /app

ARG APP_NAME=network-topology-service
ARG VERSION
ARG BUILD_TIMESTAMP
ARG COMMIT_HASH
ARG DEPLOYMENT_ENV=dev

ENV SERVICE_VERSION="${VERSION}" \
    BUILD_TIMESTAMP="${BUILD_TIMESTAMP}" \
    COMMIT_HASH="${COMMIT_HASH}" \
    ENVIRONMENT="${DEPLOYMENT_ENV}"

# Copy compiled binary
COPY --from=builder /out/${APP_NAME} /app/${APP_NAME}

# Optional certs
COPY ra-openlan-nw-topology/certs /app/certs

# Non-root user
RUN adduser -D -u 65532 appuser
USER appuser

EXPOSE 8088 17007

ENTRYPOINT ["/app/network-topology-service"]
