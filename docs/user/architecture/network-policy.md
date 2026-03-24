# Network Policies Architecture

The Telemetry module follows the principle of least privilege for network traffic. Telemetry Manager creates [NetworkPolicies](https://kubernetes.io/docs/concepts/services-networking/network-policies/) for each component it manages, restricting ingress and egress traffic to only the required connections.

![Network Policies](./../assets/networkpolicies.drawio.svg)

## Overview

Each Telemetry component has a dedicated set of NetworkPolicies that control the allowed traffic. The following list describes the policies in the diagram:

1. All Telemetry module Pods can send DNS queries on port 53 to any IP (including DNS services) and on port 8053 to kube-dns.
1. All Telemetry module Pods can communicate with any IP on port 443 (including Kubernetes API Server).
1. Any Pod in the cluster can send OTLP data to the log, metric, and trace gateways on ports 4317 (gRPC) and 4318 (HTTP).
1. Pods labeled with `networking.kyma-project.io/metrics-scraping: allowed` can scrape metrics from all Telemetry components including the Telemetry Manager on their respective metrics ports.
1. The self monitor scrapes metrics from the gateways, agents, and Fluent Bit on their metrics ports (8888 for OTel Collectors, 2020 and 2021 for Fluent Bit).
1. Telemetry Manager queries the self monitor on port 9090.
1. All gateways and agents have unrestricted egress to forward telemetry data to external or in-cluster backends.
1. The self monitor sends alerts to Telemetry Manager on port 9443.
1. The Kubernetes API server and any Pod can reach Telemetry Manager on port 9443 for admission and conversion webhooks.

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

Each gateway (log, metric, trace) has two NetworkPolicies that Telemetry Manager creates dynamically:

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

The self monitor (Prometheus) has two NetworkPolicies:

| Policy | Direction | Source / Destination                                             | Port |
|---|---|------------------------------------------------------------------|---|
| Main | Egress | Fluent Bit Pods                                                  | 2020 |
| Main | Egress | Log agent and metric agent Pods                                  | 8888 |
| Main | Egress | Gateway Pods                                                     | 8888 |
| Main | Egress | Telemetry Manager Pod                                            | 9443 |
| Metrics | Ingress | Telemetry Manager Pod                                            | 9090 |
| Metrics | Ingress | Pods with `networking.kyma-project.io/metrics-scraping: allowed` | 9090 |

## Istio Sidecar Metrics

When Istio is installed, an additional port (15090) is opened in the metrics ingress policies for the gateways, agents, and Fluent Bit. This port exposes Istio Envoy sidecar telemetry and is accessible to Pods with the `networking.kyma-project.io/metrics-scraping: allowed` label.
