# Istio Integration Details

This document provides comprehensive details about how Telemetry Manager integrates with Istio across all telemetry components. For a high-level overview, see [Istio Integration](istio-integration.md).

## Overview

Telemetry Manager automatically detects Istio by checking for Istio CRDs (`*.istio.io`) in the cluster and applies Istio-specific configurations to telemetry components. This enables secure mTLS communication for outbound data export and proper metric collection from Istio-enabled workloads.

## Component Behaviors

### OTLP Gateway

The OTLP Gateway is deployed as a DaemonSet and serves as the unified ingress point for all telemetry signals (logs, traces, and metrics).

#### Unconditional (Always Applied)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Label | `sidecar.istio.io/inject: "true"` | The OTLP Gateway always has Istio sidecar injection enabled to support outbound mTLS communication to in-cluster backends in the Istio mesh. |

#### Conditional (Only When Istio is Detected)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| PeerAuthentication | PERMISSIVE mTLS mode | This configuration allows both plain-text and mTLS connections to the OTLP Gateway. This ensures applications can send data over plain-text (because of node-local ingestion), while still supporting mTLS for any direct cross-namespace access if needed. |
| DestinationRule | `TLS mode: DISABLE` for all OTLP Services | The OTLP Gateway receives telemetry data over plain-text on the ingestion path because of node-local routing. Disabling TLS for client connections to these services ensures that other components (such as metric pipelines) can connect without requiring mTLS. |
| NetworkPolicy (Ingress) | Additionally allows traffic on Istio Envoy telemetry port (15090) | When Istio is present, the sidecar's Envoy proxy exposes metrics that need to be scraped by monitoring systems. |

### Metric Agent

The Metric Agent is deployed as a DaemonSet and scrapes Prometheus metrics from workloads in the cluster, then forwards them to configured backends.

#### Unconditional (Always Applied)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Label | `sidecar.istio.io/inject: "true"` | The Metric Agent always has Istio sidecar injection enabled to support outbound mTLS communication to in-cluster backends in the Istio mesh. |

#### Conditional (Only When Istio is Detected)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Annotation | `traffic.sidecar.istio.io/includeOutboundIPRanges: ""` | Bypasses Istio sidecar interception for most outbound traffic. This is necessary because Prometheus scraping of Istio control plane and Envoy metrics requires direct access to metric endpoints, which are not reachable through the sidecar proxy. |
| Pod Annotation | `traffic.sidecar.istio.io/includeOutboundPorts: "{backend_ports}"` | Ensures that traffic to configured backends (such as OTLP Gateway, in-cluster Prometheus, or other OTel Collectors) goes through the Istio sidecar for mTLS. The reconciliation loop populates this with the actual backend ports from MetricPipeline configurations. |
| Pod Annotation | `traffic.sidecar.istio.io/excludeInboundPorts: "8888"` | Excludes the metrics port from Istio sidecar interception, ensuring that the Metric Agent's own metrics can be scraped directly without mTLS overhead. |
| Pod Annotation | `proxy.istio.io/config` | Configures the Istio sidecar to write TLS certificates to the shared volume at `/etc/istio-output-certs`, which the Metric Agent uses for mTLS scraping of application metrics. |
| Pod Annotation | `sidecar.istio.io/userVolumeMount` | Mounts the Istio certificate volume into the Istio sidecar container. |
| NetworkPolicy (Ingress) | Additionally allows traffic on Istio Envoy telemetry port (15090) | When Istio is present, the sidecar's Envoy proxy exposes metrics that need to be scraped. |
| Volume Mount | Istio certificates volume (`/etc/istio-output-certs`) | The Metric Agent always mounts Istio certificates so it can scrape application metrics that require mTLS (when the application follows a STRICT mTLS policy). |

### OTel Log Agent

The OTel Log Agent is deployed as a DaemonSet and collects container logs using file-based collection, then forwards them to configured backends.

#### Unconditional (Always Applied)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Label | `sidecar.istio.io/inject: "true"` | The OTel Log Agent always has Istio sidecar injection enabled to support outbound mTLS communication to in-cluster backends in the Istio mesh. |

#### Conditional (Only When Istio is Detected)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| NetworkPolicy (Ingress) | Additionally allows traffic on Istio Envoy telemetry port (15090) | When Istio is present, the sidecar's Envoy proxy exposes metrics that need to be scraped by monitoring systems. |

### Fluent Bit

Fluent Bit is deployed as a DaemonSet and provides legacy log collection capabilities.

#### Unconditional (Always Applied)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Label | `sidecar.istio.io/inject: "true"` | Fluent Bit always has Istio sidecar injection enabled to support outbound mTLS communication to in-cluster backends in the Istio mesh. |

#### Conditional (Only When Istio is Detected)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| NetworkPolicy (Ingress) | Additionally allows traffic on Istio Envoy telemetry port (15090) | When Istio is present, the sidecar's Envoy proxy exposes metrics that need to be scraped by monitoring systems. |

### Self-Monitor

The Self-Monitor is a Prometheus instance deployed as a Deployment that scrapes metrics from Telemetry components for health monitoring and alerting.

#### Unconditional (Always Applied)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Label | `sidecar.istio.io/inject: "false"` | The Self-Monitor explicitly disables Istio sidecar injection because it only scrapes metrics from Telemetry components within the same namespace and does not need mTLS. Running without the sidecar reduces resource overhead. |

#### Conditional (Only When Istio is Detected)

No Istio-specific resources are created for the Self-Monitor because it explicitly disables sidecar injection.

## Summary Table

| Component | Sidecar Injection | Istio Certificates | Special Annotations | Istio-Specific Resources |
|-----------|-------------------|-------------------|---------------------|--------------------------|
| OTLP Gateway | Always enabled | Not used | None | PeerAuthentication (PERMISSIVE), DestinationRule (TLS DISABLE) |
| Metric Agent | Always enabled | Always mounted | Conditional (traffic routing) | None |
| OTel Log Agent | Always enabled | Not used | None | None |
| Fluent Bit | Always enabled | Not used | None | None |
| Self-Monitor | Always disabled | Not used | None | None |

## Related Information

- [Istio Integration Overview](istio-integration.md)
- [Configure Istio Access Logs](../collecting-logs/istio-support.md)
- [Configure Istio Tracing](../collecting-traces/istio-support.md)
- [Collect Istio Metrics](../collecting-metrics/istio-input.md)
