<p align="center">
  <img src="images/project/mango-logo.png" height="90" align="middle" alt="Mango Cloud Logo" />
</p>

# Mango Cloud Network Topology Service

## Overview
The Mango Cloud Network Topology Service is a custom microservice developed by Router Architects for Mango Cloud, an open-source OpenLAN/OpenWiFi cloud controller for managed Wi-Fi and connectivity operations. Mango Cloud is purpose-built for ISPs, MSPs, Edge AI providers, and IoT service providers.

The service builds a live network topology for OpenWiFi access points, mesh nodes, and connected clients within a Mango Cloud deployment.

## Role in Mango Cloud
Within Mango Cloud, this service serves as the central **Network Topology View Generator**.

Key integrations include:
* **Live Topology Builder**: Aggregates AP nodes, mesh links, signal-strength statistics, and client associations to render live wireless topology maps.
* **Intra-Microservice Analytics**: Gathers timepoint snapshots and association metrics directly from the Analytics Service (`owanalytics`).
* **Intra-Microservice Auth**: Integrates with the Security Service (`owsec`) to validate bearer tokens and API keys on its public interface.

### Resources
* [Mango Cloud Website](https://www.mangowifi.cloud/)
* [Mango Cloud Deployment Guide](https://github.com/routerarchitects/mango-cloud-deployment)
* [Router Architects GitHub Organization](https://github.com/routerarchitects)

## OpenAPI
The OpenAPI definition is available directly in the [GitHub repository](openapi/nwtopology.yaml).
You can load the [raw OpenAPI definition file](https://raw.githubusercontent.com/routerarchitects/ra-openlan-nw-topology/main/openapi/nwtopology.yaml) into [Swagger UI](https://petstore.swagger.io/) to view interactive API docs.

## Building & Deployment
This service is written in Go. To build and run the service, refer to the Go module settings (`go.mod`) and build configs in the [Dockerfile](./Dockerfile).

## Network Ports
The service listens on the following container ports:

| Container Port | Interface | Typical Deployment Port |
| :--- | :--- | :---: |
| `8088` | Public REST API | `16007` |
| `17007` | Private REST API | `17007` |

In the standard Mango Cloud deployment, the public listener is exposed externally on port `16007`. Port mappings may be changed through the deployment configuration.

## Historical Topology API

The service exposes `GET /api/v1/topology/history` on both the public and private authenticated listeners to generate historical network topology snapshots using a sliding window over analytics timepoints.

### Query Parameters

All durations and intervals are in seconds:

| Parameter | Type | Required | Default | Description |
| :--- | :--- | :---: | :---: | :--- |
| `boardId` | string | Yes | — | Analytics board identifier. |
| `duration` | integer | No | `1800` | Requested historical duration. |
| `windowInterval` | integer | Yes | — | Nominal window size and sliding step. |
| `jitter` | integer | No | `10` | Tolerance appended to the end of each window (`0` disables jitter). |

### Sliding Window Semantics & Limitations

- **Window boundaries**: Each snapshot covers `windowStart <= timestamp <= windowEnd + jitter`. Jitter is applied only to the end of the window, and the window advances by `windowInterval` (never `windowInterval + jitter`). Consecutive windows overlap by `jitter` seconds.
- **Complete nominal windows**: When `duration` is not divisible by `windowInterval`, the trailing partial window is omitted and `endDate` reflects the actual covered nominal end.
- **Online/offline inference**: The `connected` field in historical snapshots is inferred from whether an eligible timepoint exists in that window, not live WebSocket connectivity. Offline devices remain in `nodes` with empty `aps: []` and `mesh: []` arrays.
- **Timestamp origin**: Analytics timepoint timestamps originate from the device's `unit.localtime`, not database insertion time. Jitter tolerates reporting timestamp variation but does not guarantee delayed database inserts have completed.
- **Board membership scope**: Membership is retrieved from `owanalytics` current board device inventory (`GetDeviceInfo`). If a board monitors subvenues or historical device venue assignments changed, current membership alone cannot guarantee historical tenant isolation without upstream board/venue scoping.
- **Resource limits** (configurable via environment variables):
  - `TOPOLOGY_HISTORY_MAX_DURATION_SECONDS` (default `86400`)
  - `TOPOLOGY_HISTORY_MAX_JITTER_SECONDS` (default `300`)
  - `TOPOLOGY_HISTORY_MAX_SNAPSHOT_COUNT` (default `720`)
  - `TOPOLOGY_HISTORY_MAX_TIMEPOINTS_PER_FETCH` (default `5000`)
  - `TOPOLOGY_HISTORY_MAX_TOTAL_TIMEPOINTS` (default `100000`)
  - `TOPOLOGY_HISTORY_MAX_ESTIMATED_OUTPUT_UNITS` (default `250000`)

### Example Requests

Public API (Bearer token):

```bash
curl -k -X GET \
  'https://<topology-host>/api/v1/topology/history?boardId=<board-id>&duration=1800&windowInterval=60&jitter=10' \
  -H 'Authorization: Bearer <access-token>' \
  -H 'Accept: application/json'
```

Private API (internal service authentication):

```bash
curl -k -X GET \
  'https://<topology-host>/api/v1/topology/history?boardId=<board-id>&duration=1800&windowInterval=60&jitter=10' \
  -H 'X-INTERNAL-NAME: <internal-name>' \
  -H 'X-API-KEY: <instance-api-key>' \
  -H 'Accept: application/json'
```

## Kafka topics
This service uses Kafka primarily for service discovery and event-driven coordination with other platform services, participating on the `service_events` topic.

## Contributions
We welcome and appreciate community contributions. To contribute, please submit pull requests targeting the `main` branch.

## Additional OWSDK Microservices
Here is the index of additional OpenWiFi microservices:

| Name | Description | Link | OpenAPI |
| :--- | :--- | :---: | :---: |
| **OWSEC** | Security Service | [here](https://github.com/routerarchitects/ra-wlan-cloud-ucentralsec) | [here](https://github.com/routerarchitects/ra-wlan-cloud-ucentralsec/blob/main/openapi/owsec.yaml) |
| **OWGW** | Controller Service | [here](https://github.com/routerarchitects/ra-wlan-cloud-ucentralgw) | [here](https://github.com/routerarchitects/ra-wlan-cloud-ucentralgw/blob/main/openapi/owgw.yaml) |
| **OWFMS** | Firmware Management Service | [here](https://github.com/routerarchitects/ra-wlan-cloud-ucentralfms) | [here](https://github.com/routerarchitects/ra-wlan-cloud-ucentralfms/blob/main/openapi/owfms.yaml) |
| **OWPROV** | Provisioning Service | [here](https://github.com/routerarchitects/ra-wlan-cloud-owprov) | [here](https://github.com/routerarchitects/ra-wlan-cloud-owprov/blob/main/openapi/owprov.yaml) |
| **OWANALYTICS** | Analytics Service | [here](https://github.com/routerarchitects/ra-wlan-cloud-analytics) | [here](https://github.com/routerarchitects/ra-wlan-cloud-analytics/blob/main/openapi/owanalytics.yaml) |
| **OWSUB** | Subscriber Portal Service | [here](https://github.com/routerarchitects/ra-wlan-cloud-userportal) | [here](https://github.com/routerarchitects/ra-wlan-cloud-userportal/blob/main/openapi/userportal.yaml) |
| **NW-Topology** | Network Topology Service | [here](https://github.com/routerarchitects/ra-openlan-nw-topology) | [here](https://github.com/routerarchitects/ra-openlan-nw-topology/blob/main/openapi/nwtopology.yaml) |
