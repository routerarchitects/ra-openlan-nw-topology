# syntax=docker/dockerfile:1.6

############################
# Builder
############################
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /src

# Cache deps
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy source
COPY . .

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
# Runtime (Distroless w/ CA certs)
############################
FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /app
ARG APP_NAME=network-topology-service
COPY --from=builder "/out/${APP_NAME}" "/app/${APP_NAME}"

# include your repo's certs directory (optional if your app uses it)
COPY certs /app/certs

USER nonroot:nonroot
EXPOSE 8088 
ENTRYPOINT ["/app/network-topology-service"]

############################
# Runtime (Alpine dev shell)
############################
FROM alpine:3.20 AS runtime-alpine

# (Optional) bash; alpine already has /bin/sh (ash)
RUN apk add --no-cache ca-certificates bash curl

WORKDIR /app
ARG APP_NAME=network-topology-service
COPY --from=builder "/out/${APP_NAME}" "/app/${APP_NAME}"
# COPY certs /app/certs

# Drop privileges by creating a user if you like:
RUN adduser -D -u 65532 appuser
USER appuser

EXPOSE 17007
ENTRYPOINT ["/app/network-topology-service"]
