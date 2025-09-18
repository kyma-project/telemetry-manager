---
title: Network Policy Hardening
status: Accepted
date: 2025-09-18
---

# 28. Network Policy Hardening

## Context

| **Workload** | **Network Policy Name** | **Ingress Rules** | **Egress Rules** | **Pod Selector** |
|--------------|------------------------|-------------------|------------------|------------------|
| **Telemetry Manager** | `telemetry-manager-manager` | **From:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** 8080, 8081, 9443 (TCP) | **To:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** All | `app.kubernetes.io/instance: telemetry`<br>`app.kubernetes.io/name: manager`<br>`control-plane: telemetry-manager`<br>`kyma-project.io/component: controller` |
| **FluentBit Agent** | `telemetry-fluent-bit` | **From:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** 2020, 2021, 15090 (optional) (TCP) | **To:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** All | `app.kubernetes.io/instance: telemetry`<br>`app.kubernetes.io/name: fluent-bit` |
| **OTel Log Agent** | `telemetry-log-agent` | **From:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** 8888, 13133, 15090 (optional) (TCP) | **To:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** All | `app.kubernetes.io/name: telemetry-log-agent` |
| **OTel Log Gateway** | `telemetry-log-gateway` | **From:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** 8888, 13133, 4318, 4317, 15090 (optional) (TCP) | **To:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** All | `app.kubernetes.io/name: telemetry-log-gateway` |
| **OTel Metric Agent** | `telemetry-metric-agent` | **From:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** 8888, 13133, 15090 (optional) (TCP) | **To:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** All | `app.kubernetes.io/name: telemetry-metric-agent` |
| **OTel Metric Gateway** | `telemetry-metric-gateway` | **From:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** 8888, 13133, 4318, 4317, 15090 (optional) (TCP) | **To:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** All | `app.kubernetes.io/name: telemetry-metric-gateway` |
| **OTel Trace Gateway** | `telemetry-trace-gateway` | **From:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** 8888, 13133, 4318, 4317, 15090 (optional) (TCP) | **To:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** All | `app.kubernetes.io/name: telemetry-trace-gateway` |
| **Self Monitor** | `telemetry-self-monitor` | **From:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** 9090 (TCP) | **To:** Any IP (0.0.0.0/0, ::/0)<br>**Ports:** All | `app.kubernetes.io/name: telemetry-self-monitor` |

## Decision

What is the change that we're proposing or have agreed to implement?

## Consequences

What becomes easier or more difficult to do and any risks introduced by the change that will need to be mitigated.
