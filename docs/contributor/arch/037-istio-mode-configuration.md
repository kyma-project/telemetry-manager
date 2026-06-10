---
title: Istio Mode Configuration API
status: Proposed
date: 2026-06-10
---

# Istio Mode Configuration API

## Context and Problem Statement

Telemetry Manager currently auto-detects Istio by checking for Istio CRDs (`*.istio.io`) in the cluster and automatically applies Istio-specific resources when detected. This includes sidecar injection labels, PeerAuthentication resources, DestinationRules, traffic routing annotations, and certificate volume mounts across telemetry components (OTLP Gateway, Metric Agent, OTel Log Agent, Fluent Bit, and Self-Monitor).

While automatic detection provides convenience, it creates several operational challenges:

- No explicit control: Users cannot disable Istio mode even when it is not needed, resulting in unnecessary resource overhead from sidecar containers, certificate management, and Istio-specific network policies.

- Unconditional artifacts: Some Istio-related behaviors remain applied unconditionally regardless of detection. For example, all components (except Self-Monitor) always have `sidecar.istio.io/inject: "true"` and the Metric Agent always mounts Istio certificate volumes, even when Istio integration is not required.

- All-or-nothing approach: The current design does not allow granular control. Users cannot selectively enable Istio mode for specific components or use cases (such as backends requiring in-cluster mTLS).

- Migration difficulty: Moving toward an "Istio mode OFF by default" model (tracked in [issue #657](https://github.com/kyma-project/telemetry-manager/issues/657)) requires a backward-compatible transition path that preserves existing behavior while allowing explicit opt-in configuration.

The system needs an API mechanism to explicitly enable or disable Istio mode, providing users with control over when and how Istio integration is applied to telemetry components. This addresses [issue #3549](https://github.com/kyma-project/telemetry-manager/issues/3549), which proposes an explicit configuration model that supports eventual migration to an opt-in default while maintaining backward compatibility during the transition.

See [Istio Integration Details](../../user/architecture/istio-integration-details.md) for a complete mapping of current Istio behaviors across all telemetry components.


## Current Istio Integration

### Istio Detection

The Telemetry Manager automatically detects Istio by checking for the presence of Istio CRDs (Custom Resource Definitions) in the cluster. Specifically, it looks for API groups matching `*.istio.io`.

### Component Behaviors

#### OTLP Gateway

The OTLP Gateway is deployed as a DaemonSet and serves as the unified ingress point for all telemetry signals (logs, traces, and metrics).

##### Unconditional (Always Applied)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Label | `sidecar.istio.io/inject: "true"` | The OTLP Gateway always has Istio sidecar injection enabled to support outbound mTLS communication to in-cluster backends that are part of the Istio mesh. |

##### Conditional (Only When Istio is Detected)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| PeerAuthentication | PERMISSIVE mTLS mode | Allows both plain-text and mTLS connections to the OTLP Gateway. This ensures applications can send data over plain-text (because of node-local ingestion), while still supporting mTLS for any direct cross-namespace access if needed. |
| DestinationRule | `TLS mode: DISABLE` for all OTLP Services | The OTLP Gateway receives telemetry data over plain-text on the ingestion path because of node-local routing. Disabling TLS for client connections to these services ensures that other components (such as metric pipelines) can connect without requiring mTLS. |
| NetworkPolicy (Ingress) | Additionally allows traffic on Istio Envoy telemetry port (15090) | When Istio is present, the sidecar's Envoy proxy exposes metrics that need to be scraped by monitoring systems. |

#### Metric Agent

The Metric Agent is deployed as a DaemonSet and scrapes Prometheus metrics from workloads in the cluster, then forwards them to configured backends.

##### Unconditional (Always Applied)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Label | `sidecar.istio.io/inject: "true"` | The Metric Agent always has Istio sidecar injection enabled to support outbound mTLS communication to in-cluster backends that are part of the Istio mesh. |

##### Conditional (Only When Istio is Detected)

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

##### Unconditional (Always Applied)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Label | `sidecar.istio.io/inject: "true"` | The OTel Log Agent always has Istio sidecar injection enabled to support outbound mTLS communication to in-cluster backends that are part of the Istio mesh. |

##### Conditional (Only When Istio is Detected)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| NetworkPolicy (Ingress) | Additionally allows traffic on Istio Envoy telemetry port (15090) | When Istio is present, the sidecar's Envoy proxy exposes metrics that need to be scraped by monitoring systems. |

#### Fluent Bit

Fluent Bit is deployed as a DaemonSet and provides legacy log collection capabilities.

##### Unconditional (Always Applied)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Label | `sidecar.istio.io/inject: "true"` | Fluent Bit always has Istio sidecar injection enabled to support outbound mTLS communication to in-cluster backends that are part of the Istio mesh. |

##### Conditional (Only When Istio is Detected)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| NetworkPolicy (Ingress) | Additionally allows traffic on Istio Envoy telemetry port (15090) | When Istio is present, the sidecar's Envoy proxy exposes metrics that need to be scraped by monitoring systems. |

### Self-Monitor

The Self-Monitor is a Prometheus instance deployed as a Deployment that scrapes metrics from Telemetry components for health monitoring and alerting.

##### Unconditional (Always Applied)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Label | `sidecar.istio.io/inject: "false"` | The Self-Monitor explicitly disables Istio sidecar injection because it only scrapes metrics from Telemetry components within the same namespace and does not need mTLS. Running without the sidecar reduces resource overhead. |


##### Conditional (Only When Istio is Detected)

No Istio-specific resources are created for the Self-Monitor because it explicitly disables sidecar injection.

## Summary Table

| Component | Sidecar Injection | Istio Certificates | Special Annotations | Istio-Specific Resources |
|-----------|-------------------|-------------------|---------------------|--------------------------|
| OTLP Gateway | ✅ Always enabled | ❌ Not used | ❌ None | PeerAuthentication (PERMISSIVE), DestinationRule (TLS DISABLE) |
| Metric Agent | ✅ Always enabled | ✅ Always mounted | ✅ Conditional (traffic routing) | None |
| OTel Log Agent | ✅ Always enabled | ❌ Not used | ❌ None | None |
| Fluent Bit | ✅ Always enabled | ❌ Not used | ❌ None | None |
| Self-Monitor | ❌ Always disabled | ❌ Not used | ❌ None | None |






