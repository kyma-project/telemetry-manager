# Network Policies Architecture

The Telemetry module follows the principle of least privilege for network traffic. Telemetry Manager creates [NetworkPolicies](https://kubernetes.io/docs/concepts/services-networking/network-policies/) for each component it manages, restricting ingress and egress traffic to only the required connections.

![Network Policies](../assets/networkpolicies.drawio.svg)

## Overview

Each Telemetry component has a dedicated set of NetworkPolicies that control the allowed traffic. The following items describe the policies in the diagram:

1. All Telemetry module Pods can send DNS queries to any IP on port 53, including DNS services, and to kube-dns on port 8053.
2. All Telemetry module Pods can connect to any IP on port 443, including the Kubernetes API server.
3. Any Pod in the cluster can send OTLP data to the log, metric, and trace gateways on ports 4317 for gRPC and 4318 for HTTP.
4. Pods with the `networking.kyma-project.io/metrics-scraping: allowed` label can scrape metrics from all Telemetry components, including Telemetry Manager, on their respective metrics ports.
5. The self monitor can scrape metrics from the gateways, agents, and Fluent Bit on ports 8888 for OTel Collectors, and 2020 for Fluent Bit.
6. Telemetry Manager can query the self monitor on port 9090.
7. All gateways and agents can forward telemetry data to external or in-cluster backends on any port.
8. The self monitor can send alerts to Telemetry Manager on port 9443.
9. The Kubernetes API server and any Pod can reach Telemetry Manager for webhooks on port 9443.

## Telemetry Manager Policies

Telemetry Manager has the following NetworkPolicies, deployed with the Helm chart:

| Policy | Direction | Source / Destination | Port |
|---|---|---|---|
| Manager main | Egress | Self monitor Pods | 9090 |
| Manager to DNS | Egress | DNS services | 53, 8053 |
| Manager to API server | Egress | Kubernetes API server | 443 |
| Manager webhooks | Ingress | Any source | 9443 |
| Manager metrics | Ingress | Pods with `networking.kyma-project.io/metrics-scraping: allowed` | 8080 |

## Gateway Policies

The log, metric, and trace gateways each have two NetworkPolicies that Telemetry Manager creates dynamically:

| Policy                          | Direction | Source / Destination | Port |
|---------------------------------|---|---|---|
| General                         | Egress | Any destination | Unrestricted |
| OTLP ingress (part of General)  | Ingress | Any source | 4317, 4318 |
| Metrics                         | Ingress | Pods with `networking.kyma-project.io/metrics-scraping: allowed` | 8888 |

The OTLP ingress policy does not restrict the source, so any Pod in the cluster can push telemetry data to the gateways.

## Agent Policies

Each OTel Collector agent (log agent, metric agent) has two NetworkPolicies:

| Policy | Direction | Source / Destination | Port |
|---|---|---|---|
| General | Egress | Any destination | Unrestricted |
| Metrics | Ingress | Pods with `networking.kyma-project.io/metrics-scraping: allowed` | 8888 |

The unrestricted egress is required because agents forward collected data to the configured backend, which can be any in-cluster or external destination.

## Fluent Bit Policies

Fluent Bit has two NetworkPolicies:

| Policy | Direction | Source / Destination | Port |
|---|---|---|---|
| General | Egress | Any destination | Unrestricted |
| Metrics | Ingress | Pods with `networking.kyma-project.io/metrics-scraping: allowed` | 2020, 2021 |

## Self Monitor Policies

The self monitor, based on Prometheus, has two NetworkPolicies:

| Policy | Direction | Source / Destination                                             | Port |
|---|---|------------------------------------------------------------------|---|
| Main | Egress | Fluent Bit Pods                                                  | 2020 |
| Main | Egress | Log agent and metric agent Pods                                  | 8888 |
| Main | Egress | Gateway Pods                                                     | 8888 |
| Main | Egress | Telemetry Manager Pod                                            | 9443 |
| Metrics | Ingress | Telemetry Manager Pod                                            | 9090 |
| Metrics | Ingress | Pods with `networking.kyma-project.io/metrics-scraping: allowed` | 9090 |

## Istio Sidecar Metrics

When Istio is installed, the metrics ingress policies for the gateways, agents, and Fluent Bit include an additional port 15090. This port exposes Istio Envoy sidecar telemetry to Pods with the `networking.kyma-project.io/metrics-scraping: allowed` label.
