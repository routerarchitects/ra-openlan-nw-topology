# OpenLAN Network Topology Service

## What is it?
The OpenLAN Network Topology Service is a microservice that builds a live wireless topology view for a given board.
It consumes device and timepoint information from dependent OpenWiFi services, correlates AP and mesh interfaces,
attaches client associations, and returns a normalized topology graph over a secure REST API.

Like other OpenWiFi-style services, it runs as a TLS-protected service, participates in service discovery, and uses authentication patterns for both public and private access.

## OpenAPI
This service is defined with an API document available in this repository at `/openapi/nwtopology.yaml`


## Docker
To use the CLoudSDK deployment please follow [here](https://github.com/routerarchitects/mango-cloud-deployment)

## Kafka topics
This service uses Kafka primarily for service discovery and coordination with other platform services , utilizing the `service_events` topic