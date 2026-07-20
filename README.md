<p align="center">
  <img src="images/project/mango-logo.png" height="90" align="middle" alt="Mango Cloud Logo" />
</p>

# Mango Cloud Network Topology Service

## Overview
The Mango Cloud Network Topology Service is a custom microservice developed by Router Architects specifically for the Mango Cloud ecosystem. It builds a live network topology visualization for active nodes within your deployment.

By consuming device state and timepoint information from dependent services (such as `owanalytics` and `owgw`), the service correlates AP-to-AP mesh interfaces, attaches client association links, and returns a normalized, structured topology graph over a secure REST API.

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

## Firewall Considerations
The microservice exposes two main network ports. Ensure your firewall or container ingress rules are configured accordingly:

| Port | Description | Configurable |
| :--- | :--- | :---: |
| `16007` | Default public REST API access port (often mapped from internal port `8088`) | yes |
| `17007` | Default internal/private REST API access port | yes |

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
