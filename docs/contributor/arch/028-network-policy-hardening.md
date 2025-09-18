---
title: Network Policy Hardening
status: Accepted
date: 2025-09-18
---

# 28. Network Policy Hardening

## Context

### Communication Flow Analysis

#### FluentBit, Log Agent:
Ingress: Metric scraping (self-monitoring and RMA) and health checks
Egress: Kubernetes API, DNS, external logging services (e.g., CLS)

#### Log, Trace, Metric Gateway:
Ingress: Metric scraping (self-monitoring and RMA), health checks, OTLP data ingested by customer workloads
Egress: Kubernetes API, DNS, external telemetry backends (e.g., CLS, Dynatrace)

#### Metric Agent:
Ingress: Metric scraping (self-monitoring and RMA), health checks
Egress: Kubernetes API, DNS, Kubelet, scraping customer workloads metrics, external telemetry backends (e.g., CLS, Dynatrace)

#### Self-Monitor:
Ingress: Metric scraping (self-monitoring and RMA), health checks, alert queries
Egress: Kubernetes API, DNS, scraping module components metrics

#### Telemetry Manager:
Ingress: Metric scraping (RMA), health checks, alertmanager webhook, admission and conversion webhooks
Egress: Kubernetes API, DNS, self-monitor alert queries

### Current Network Policies

| **Workload** | **Network Policy Name** | **Ingress Rules** | **Egress Rules** | **Pod Selector** |
|--------------|------------------------|-------------------|------------------|------------------|
| **Telemetry Manager** | `telemetry-manager-manager` | **From:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** 8080, 8081, 9443 | **To:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** All | `app.kubernetes.io/instance: telemetry`<br>`app.kubernetes.io/name: manager`<br>`control-plane: telemetry-manager`<br>`kyma-project.io/component: controller` |
| **FluentBit Agent** | `telemetry-fluent-bit` | **From:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** 2020, 2021, 15090 (optional) | **To:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** All | `app.kubernetes.io/instance: telemetry`<br>`app.kubernetes.io/name: fluent-bit` |
| **OTel Log Agent** | `telemetry-log-agent` | **From:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** 8888, 13133, 15090 (optional) | **To:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** All | `app.kubernetes.io/name: telemetry-log-agent` |
| **OTel Log Gateway** | `telemetry-log-gateway` | **From:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** 8888, 13133, 4318, 4317, 15090 (optional) | **To:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** All | `app.kubernetes.io/name: telemetry-log-gateway` |
| **OTel Metric Agent** | `telemetry-metric-agent` | **From:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** 8888, 13133, 15090 (optional) | **To:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** All | `app.kubernetes.io/name: telemetry-metric-agent` |
| **OTel Metric Gateway** | `telemetry-metric-gateway` | **From:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** 8888, 13133, 4318, 4317, 15090 (optional) | **To:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** All | `app.kubernetes.io/name: telemetry-metric-gateway` |
| **OTel Trace Gateway** | `telemetry-trace-gateway` | **From:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** 8888, 13133, 4318, 4317, 15090 (optional) | **To:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** All | `app.kubernetes.io/name: telemetry-trace-gateway` |
| **Self-Monitor** | `telemetry-self-monitor` | **From:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** 9090 (TCP) | **To:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** All | `app.kubernetes.io/name: telemetry-self-monitor` |

## Decision

### Issues/questions to address

- Our network policies (loose) are already present, so `enableNetworkPolicies` toggle does not make sense for us.
- No egress rules, too loose. Only ingress ports are limited at the moment.
- There is a need to define cross-workload policies applied based on labels. For example, every Telemetry module component should be able to communicate with the API server, DNS, and other essential services. And such a policy have to be defined in a single place, not per each workload. Health checks?
- Egress when reaching external services (CLS, Dynatrace, etc.).
- RMA/Prometheus scraping.
- Telemetry module collecting metrics from other modules?


## Consequences

What becomes easier or more difficult to do and any risks introduced by the change that will need to be mitigated.
