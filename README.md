RA OpenLAN Network Topology Service
===================================

This service exposes a TLS-protected HTTP API that builds the current wireless topology for a given board. It:
- Uses service discovery (Kafka-backed) to find dependent services and announce itself (`ow-common-mods/servicediscovery`).
- Calls discovered services (e.g., `owanalytics`, `owsec`) through a shared OpenAPI client with internal headers (`adapters/httpclient`, `internal/gateway`).
- Builds a topology graph from timepoint samples: normalizes BSSIDs, separates AP vs mesh faces, attaches clients, and derives mesh edges (`internal/services/topology_service.go`).
- Serves `/api/v1/topology` behind internal header or bearer-token validation (`internal/api`, `internal/api/middlewares`), plus `/livez`.

HTTP API
--------
- `GET /api/v1/topology?boardId={id}` (see `internal/models.TimepointsQuery`)
  - Auth:
    - Internal: include `X-INTERNAL-NAME: <anything>` and `X-API-KEY: sha256(PUBLIC_ENDPOINT)`.
    - External: `Authorization: Bearer <token>` validated via `owsec`.
  - Response: `models.Topology` JSON with nodes (devices + faces), mesh edges, and timestamp.
    - Face timestamps are emitted in Asia/Kolkata; overall topology timestamp is UTC.

Configuration (env vars)
------------------------
Key settings parsed in `internal/config/config.go` (defaults in code):
- Server/TLS:
  - `HTTP_PORT` (default `8088`)
  - `PRIVATE_HTTP_PORT` (default `17007`)
  - `INTERNAL_RESTAPI_HOST_CERT` / `INTERNAL_RESTAPI_HOST_KEY` (TLS cert/key; required to start)
  - `INTERNAL_RESTAPI_HOST_ROOTCA` (optional CA for outbound TLS from the HTTP client)
- Kafka:
  - `KAFKA_BROKERS` (comma separated)
  - `KAFKA_CLIENT_ID`
  - `KAFKA_TOPIC_CMD`
  - `KAFKA_GROUP_ID`
  - `KAFKA_INITIAL_OFFSET`
  - `KAFKA_SESSION_TIMEOUT`
  - `KAFKA_HEARTBEAT_INTERVAL`
  - `KAFKA_MAX_RETRIES`
  - `KAFKA_PRODUCER_TIMEOUT`
  - `KAFKA_REQUIRED_ACKS`
  - `KAFKA_INDEMPOTENT`
- Service discovery:
  - `DISCOVERY_TOPIC`
  - `DISCOVERY_SERVICE_TYPE`
  - `DISCOVERY_SERVICE_VERSION`
  - `DISCOVERY_PRIVATE_ENDPOINT`
  - `DISCOVERY_PUBLIC_ENDPOINT`
  - `DISCOVERY_INSTANCE_ID` (optional)
  - `DISCOVERY_INSTANCE_KEY` (optional)
  - `DISCOVERY_KEEPALIVE_INTERVAL`
  - `DISCOVERY_EXPIRY_MULTIPLIER`
  - `DISCOVERY_SWEEP_INTERVAL`
  - `DISCOVERY_ORDERING`
- Logging:
  - `SERVICE_NAME`
  - `SERVICE_VERSION`
  - `ENVIRONMENT`
  - `LOG_FORMAT`
  - `LOG_LEVEL`
  - `LOG_SUBSYSTEM_LEVELS`
  - `LOG_REDACT_ENABLED`
  - `LOG_REDACT_KEYS`
  - `LOG_REDACT_REPLACEMENT`
  - `LOG_STACK_ENABLED`
  - `LOG_STACK_LEVEL`

Sample local env (adapt paths/brokers as needed):
```
export HTTP_PORT=8088
export PRIVATE_HTTP_PORT=17007
export INTERNAL_RESTAPI_HOST_CERT=./certs/restapi-cert.pem
export INTERNAL_RESTAPI_HOST_KEY=./certs/restapi-key.pem
export INTERNAL_RESTAPI_HOST_ROOTCA=./certs/restapi-ca.pem

export DISCOVERY_TOPIC=service_events
export DISCOVERY_SERVICE_TYPE=nwtopology
export DISCOVERY_SERVICE_VERSION=v1
export DISCOVERY_PRIVATE_ENDPOINT=https://127.0.0.1:17007
export DISCOVERY_PUBLIC_ENDPOINT=https://127.0.0.1:8088
export DISCOVERY_ORDERING=last-seen

export KAFKA_BROKERS=localhost:9092
export KAFKA_GROUP_ID=nwtopology-service-group
export KAFKA_TOPIC_CMD=service_event

export SERVICE_NAME=network-topology-service
export SERVICE_VERSION=dev
export ENVIRONMENT=dev
export LOG_FORMAT=text
export LOG_LEVEL=info
```

Build & Run Locally (Go 1.25+)
------------------------------
1) Install deps: `go mod tidy`.
2) Build: `go build -o bin/nw-topology ./cmd/main.go`.
3) Run with env set (TLS cert/key are mandatory): `./bin/nw-topology` or `go run ./cmd/main.go`.
4) Test the API (replace board/token):
   `curl -k -H "Authorization: Bearer <TOKEN>" "https://localhost:8088/api/v1/topology?boardId=<BOARD_ID>"`.
5) Validate: `go test ./...` (also runs during Docker build).

Container Builds
----------------
- Docker: `docker build -t network-topology .` then
  `docker-compose up`.
- Compose uses `settings.local.env` and mounted certs if you provide them.

Code Map (quick pointers)
-------------------------
- `cmd/main.go`: wiring/bootstrap, TLS HTTP server, Kafka producers/consumer, lifecycle service start.
- `internal/api`: Fiber server setup, auth middleware, route registration, topology handler.
- `internal/services`: topology builder logic.
- `internal/gateway`: owanalytics and owsec clients built on service discovery.
- `adapters/httpclient`: HTTP client for internal service-to-service calls.
