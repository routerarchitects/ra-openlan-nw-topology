# syntax=docker/dockerfile:1.6

############################
# Builder
############################
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /src/ra-openlan-nw-topology

# Cache deps
COPY uttam-repos/ra-openlan-nw-topology/go.mod uttam-repos/ra-openlan-nw-topology/go.sum ./
# Local module replacements (match go.mod replace paths)
COPY ra-github-public-repos/ra-common-mods/kafka /ra-github-public-repos/ra-common-mods/kafka
COPY ra-github-public-repos/ra-common-mods/logger /ra-github-public-repos/ra-common-mods/logger
COPY ra-github-public-repos/ra-common-mods/logger-routes /ra-github-public-repos/ra-common-mods/logger-routes
COPY ra-github-public-repos/ow-common-mods/service-discovery /ra-github-public-repos/ow-common-mods/service-discovery
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy source
COPY uttam-repos/ra-openlan-nw-topology/. .

# Build
ARG MAIN=./cmd            # because you have cmd/main.go
ARG APP_NAME=network-topology-service
ENV CGO_ENABLED=0 GOOS=linux GOFLAGS=-buildvcs=false

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go test ./...

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o "/out/${APP_NAME}" "${MAIN}"

############################
# Runtime Stage
############################
FROM alpine:3.20

RUN apk add --no-cache ca-certificates bash curl

WORKDIR /app

ARG APP_NAME=network-topology-service

# Copy compiled binary
COPY --from=builder /out/${APP_NAME} /app/${APP_NAME}

# Optional certs
COPY uttam-repos/ra-openlan-nw-topology/certs /app/certs

# Non-root user
RUN adduser -D -u 65532 appuser
USER appuser

EXPOSE 8088 17007

ENTRYPOINT ["/app/network-topology-service"]
