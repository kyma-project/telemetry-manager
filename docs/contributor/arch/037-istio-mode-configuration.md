---
title: Istio Mode Configuration API
status: Proposed
date: 2026-06-10
related:
  - https://github.com/kyma-project/telemetry-manager/issues/3549
  - https://github.com/kyma-project/telemetry-manager/issues/657
---

# Istio Mode Configuration API

## Context and Problem Statement

Telemetry Manager currently auto-detects Istio by checking for Istio CRDs (`*.istio.io`) in the cluster and automatically applies Istio-specific resources when detected. This includes sidecar injection labels, DestinationRules, traffic routing annotations, and certificate volume mounts across telemetry components (OTLP Gateway, Metric Agent, OTel Log Agent, Fluent Bit, and Self-Monitor).

Automatic detection provides convenience. However, users face several operational challenges:

- No explicit control: Users have no way to disable Istio mode when they don't need it. This causes sidecar containers, certificate management, and Istio-specific network policies to create unnecessary resource overhead.

- Unconditional artifacts: Telemetry Manager applies some Istio-related behaviors unconditionally, regardless of whether it detects Istio. For example, all components (except Self-Monitor) always have `sidecar.istio.io/inject: "true"` and the Metric Agent always mounts Istio certificate volumes, even when Istio integration is not required.

- All-or-nothing approach: The current design does not allow granular control. Users cannot selectively enable Istio mode for specific components or use cases (such as backends requiring in-cluster mTLS).

- Migration difficulty: To move toward an "Istio mode off by default" model (tracked in issue [#657](https://github.com/kyma-project/telemetry-manager/issues/657)), the system requires a backward-compatible transition path that preserves existing behavior while allowing explicit opt-in configuration.

Users need an explicit API mechanism to control when and how Istio integration applies to telemetry components. This addresses issue [#3549](https://github.com/kyma-project/telemetry-manager/issues/3549), which proposes an explicit configuration model that supports eventual migration to an opt-in default while maintaining backward compatibility during the transition.

### Current Behavior

### Istio Detection

The Telemetry Manager automatically detects Istio by checking for the presence of Istio CRDs (Custom Resource Definitions) in the cluster. Specifically, it looks for API groups matching `*.istio.io`.

### Component Behaviors

#### OTLP Gateway

The OTLP Gateway is deployed as a DaemonSet and serves as the unified ingress point for all telemetry signals (logs, traces, and metrics).

##### Unconditional (Always Applied)

| Resource  | Behavior                          | Rationale                                                                                                                                       |
|-----------|-----------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Label | `sidecar.istio.io/inject: "true"` | The Gateway forwards telemetry data to backends. When backends are in-cluster services within the Istio mesh with STRICT mTLS policies, the Gateway needs the sidecar to establish mTLS connections for output. |
| OTel Processor          | `istio_enrichment` (log pipelines only when OTel log agent is used) | Enriches log entries with Istio service mesh metadata (service names, workload identifiers, mesh configuration). Only applied to log pipelines when using the OTel log agent (not Fluent Bit).                                                                                                 |
| OTel Processor          | `istio_noise_filter` (all pipelines: logs, traces, metrics)        | Filters out noisy Istio-related telemetry data (health check spans, internal mesh metrics, verbose Envoy logs) to reduce data volume and improve signal quality.                                                                                                                              |     

##### Conditional (Only When Istio is Detected)

| Resource                | Behavior                                                           | Rationale                                                                                                                                                                                                                                                                                     | 
|-------------------------|--------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Annotation          | `sidecar.istio.io/interceptionMode: TPROXY`                        | This annotation is ineffective in the current configuration because `traffic.sidecar.istio.io/includeInboundPorts: ""` disables all inbound interception. TPROXY only affects traffic that is intercepted by the sidecar.                                                                     |
| Pod Annotation          | `traffic.sidecar.istio.io/includeInboundPorts: ""`                 | The Gateway receives telemetry data on OTLP ports (4317, 4318) from both in-mesh and out-of-mesh clients. Excluding all inbound ports from sidecar interception allows direct connections without requiring mTLS, improving ingestion performance.                               | 
| PeerAuthentication      | PERMISSIVE mTLS mode                                               | This resource is ineffective in the current configuration because `traffic.sidecar.istio.io/includeInboundPorts: ""` bypasses all inbound traffic interception. The sidecar never processes inbound OTLP traffic, so the PeerAuthentication policy has no effect. The Gateway receives all telemetry data directly without mTLS enforcement.                                        |        
| DestinationRule         | `TLS mode: DISABLE` for all OTLP Services                          | The Gateway Services (OTLP ingress endpoints) use node-local routing where clients connect to the same-node Gateway pod without encryption. Application workloads must reach these endpoints over plain-text, so TLS is disabled for client-to-Gateway connections. |      
| NetworkPolicy (Ingress) | Additionally permits traffic on Istio Envoy telemetry port (15090) | The Gateway's sidecar exposes Envoy metrics on port 15090. Monitoring systems with the pod label `networking.kyma-project.io/metrics-scraping: allowed` can scrape these metrics to observe the Gateway's sidecar metrics and health.                                                         |     

#### Metric Agent

The Metric Agent is deployed as a DaemonSet and collects Prometheus, runtime or istio metrics, then forwards them to configured backends.

##### Unconditional (Always Applied)

| Resource                 | Behavior                                                                           | Rationale                                                                                                                                                                                                                                                    |
|--------------------------|------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Label                | `sidecar.istio.io/inject: "true"`                                                  | The Metric Agent forwards scraped metrics to backends. When backends are in-cluster services within the Istio mesh with STRICT mTLS policies, the Agent needs the sidecar to establish mTLS connections for output. Additionally, when Prometheus input is enabled, the sidecar injects Istio certificates that the Agent uses to scrape workloads with STRICT mTLS policies.                                          |
| Prometheus Scrape Config (Prometheus Input Enabled) | `app-services` job scrapes services with `prometheus.io/scrape: "true"` annotation | The Metric Agent must scrape application metrics from workloads that expose plain-text Prometheus endpoints. When Prometheus input is enabled, this job drops HTTPS targets (identified by `security.istio.io/tlsMode: istio` pod label or `prometheus.io/scheme: https` service annotation) because those require mTLS certificates and are handled by the `app-services-secure` job instead. |
| OTel Processor (Istio Input Enabled) | `istio_noise_filter` (metric pipelines)                                                               | Filters out noisy Istio-related metrics (verbose Envoy metrics, internal mesh control plane data) to reduce data volume and improve signal quality for metric pipelines.                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |

##### Conditional (Only When Istio is Detected)

| Resource                 | Behavior                                                                                              | Rationale                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
|--------------------------|-------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Annotation           | `traffic.sidecar.istio.io/includeOutboundIPRanges: ""`                                                | This annotation disables Istio outbound traffic interception for scraping Envoy and application metrics via the Prometheus receiver. For Envoy metrics, there is no mTLS support. For in-mesh apps, the Agent uses manual mTLS with injected client certificates (automatic mTLS with interception does not work because the Agent calls pod IPs directly). For out-of-mesh apps, mTLS is not needed.                                                          |
| Pod Annotation           | `traffic.sidecar.istio.io/includeOutboundPorts: "{backend_ports}"`                                    | The Metric Agent forwards scraped metrics to configured backends. When backends are in-cluster services in the Istio mesh, traffic to their ports must route through the sidecar for mTLS. The Agent dynamically populates this annotation with backend ports from MetricPipeline configurations. Setting this annotation on its own doesn't mean that outbound traffic to other ports will be excluded from being routed through the sidecar, so we still need to set the annotation `traffic.sidecar.istio.io/includeOutboundIPRanges: ""`. For more details, check https://github.com/istio/istio/issues/32677#issuecomment-2489911313 |
| Pod Annotation           | `traffic.sidecar.istio.io/excludeInboundPorts: "8888"`                                                | The Metric Agent exposes its own Prometheus metrics on port 8888. Monitoring systems (Self-Monitor) need to scrape these metrics directly without requiring mTLS, so this port is excluded from sidecar interception.                                                                                                                                                                                                                                                                                                                                                                                                                     |
| Pod Annotation           | `proxy.istio.io/config`                                                                               | The Metric Agent needs Istio certificates to scrape application workloads with STRICT mTLS policies. This annotation configures the sidecar to write certificates to a shared volume at `/etc/istio-output-certs` where the Agent can read them.                                                                                                                                                                                                                                                                                                                                                                                          |
| Pod Annotation           | `sidecar.istio.io/userVolumeMount`                                                                    | This annotation mounts the Istio certificate volume into the sidecar container so the sidecar can write certificates to the shared location.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| Prometheus Scrape Config (Prometheus Input Enabled) | `app-services-secure` job scrapes HTTPS targets with TLS config pointing to `/etc/istio-output-certs` | The Metric Agent must scrape application workloads with STRICT mTLS policies that require client certificates. This job uses the Istio certificates from the sidecar to authenticate when scraping these protected endpoints, keeping only HTTPS targets.                                                                                                                                                                                                                                                                                                                                                                                 |
| Volume Mount             | Istio certificates volume (`/etc/istio-output-certs`)                                                 | The Metric Agent needs access to Istio-provided mTLS certificates to scrape workloads with STRICT mTLS policies. The system mounts this shared volume when Istio is detected and Prometheus input is enabled.                                                                                                                                                                                                                                                                                                                                                                                                                             |
| NetworkPolicy (Ingress)  | Additionally permits traffic on Istio Envoy telemetry port (15090)                                    | The Metric Agent's sidecar exposes Envoy metrics on port 15090. Monitoring systems with the pod label `networking.kyma-project.io/metrics-scraping: allowed` can scrape these metrics to observe the Agent's outbound mTLS connections and sidecar health.                                                                                                                                                                                                                                                                                                                                                                                |

**Key Behavior**: The `app-services` and `app-services-secure` jobs work together to provide complete coverage:
- `app-services`: Configured when Prometheus input is enabled. Scrapes workloads that expose plain-text Prometheus metrics (drops `https` targets).
- `app-services-secure`: Only configured when Istio is detected and Prometheus input is enabled. Scrapes workloads with STRICT mTLS policies using Istio certificates (keeps only `https` targets).

This complementary design ensures each workload is scraped by exactly one job, preventing duplicate metrics.


#### OTel Log Agent

The OTel Log Agent is deployed as a DaemonSet and collects container logs using file-based collection, then forwards them to configured backends.

##### Unconditional (Always Applied)

| Resource       | Behavior                                               | Rationale                                                                                                                                                          |
|----------------|--------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Label      | `sidecar.istio.io/inject: "true"`                      | The OTel Log Agent forwards collected logs to backends. When backends are in-cluster services within the Istio mesh with STRICT mTLS policies, the Agent needs the sidecar to establish mTLS connections for output.                  |
| Pod Annotation | `traffic.sidecar.istio.io/excludeInboundPorts: "8888"` | The OTel Log Agent exposes its own Prometheus metrics on port 8888. Monitoring systems (Self-Monitor) need to scrape these metrics directly without requiring mTLS, so this port is excluded from sidecar interception. |

##### Conditional (Only When Istio is Detected)

| Resource                | Behavior                                                          | Rationale                                                                                             |
|-------------------------|-------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------|
| NetworkPolicy (Ingress) | Additionally permits traffic on Istio Envoy telemetry port (15090) | The OTel Log Agent's sidecar exposes Envoy metrics on port 15090. Monitoring systems with the pod label `networking.kyma-project.io/metrics-scraping: allowed` can scrape these metrics to observe the Agent's outbound mTLS connections and sidecar health. |

#### Fluent Bit

Fluent Bit is deployed as a DaemonSet and provides legacy log collection capabilities.

##### Unconditional (Always Applied)

| Resource  | Behavior                          | Rationale                                                                                                                                 |
|-----------|-----------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Label | `sidecar.istio.io/inject: "true"` | Fluent Bit forwards collected logs to backends. When backends are in-cluster services within the Istio mesh with STRICT mTLS policies, Fluent Bit needs the sidecar to establish mTLS connections for output. |

##### Conditional (Only When Istio is Detected)

| Resource                | Behavior                                                          | Rationale                                                                                                                                                      |
|-------------------------|-------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Annotation          | `traffic.sidecar.istio.io/excludeInboundPorts: "2020, 2021"`      | Fluent Bit exposes its own metrics on ports 2020 and 2021. Monitoring systems (Self-Monitor) need to scrape these metrics directly without requiring mTLS, so these ports are excluded from sidecar interception. |
| NetworkPolicy (Ingress) | Additionally permits traffic on Istio Envoy telemetry port (15090) | Fluent Bit's sidecar exposes Envoy metrics on port 15090. Monitoring systems with the pod label `networking.kyma-project.io/metrics-scraping: allowed` can scrape these metrics to observe Fluent Bit's outbound mTLS connections and sidecar health.                                                          |


#### Self-Monitor

The Self-Monitor is a Prometheus instance deployed as a Deployment that scrapes metrics from Telemetry components for health monitoring and alerting.

##### Unconditional (Always Applied)

| Resource  | Behavior                           | Rationale                                                                                                                                                                                                                           |
|-----------|------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Label | `sidecar.istio.io/inject: "false"` | The Self-Monitor scrapes metrics only from Telemetry components within the same namespace (kyma-system) over plain-text connections. These targets do not require mTLS authentication. Disabling sidecar injection reduces resource overhead since the Self-Monitor never needs to communicate with in-mesh services. |


##### Conditional (Only When Istio is Detected)

The system creates no Istio-specific resources for the Self-Monitor because it explicitly disables sidecar injection.

### Summary

| Component      | Sidecar Injection | Istio Certificates             | Special Annotations           | Istio-Specific Resources                                               |
|----------------|-------------------|--------------------------------|-------------------------------|------------------------------------------------------------------------|
| OTLP Gateway   | Always enabled    | Not used                       | None                          | DestinationRule (TLS DISABLE), PeerAuthentication PERMISSIVE mTLS mode |
| Metric Agent   | Always enabled    | Conditional (Prometheus input) | Conditional (traffic routing) | None                                                                   |
| OTel Log Agent | Always enabled    | Not used                       | None                          | None                                                                   |
| Fluent Bit     | Always enabled    | Not used                       | None                          | None                                                                   |
| Self-Monitor   | Always disabled   | Not used                       | None                          | None                                                                   |

## Considered Option

### Single `trafficInterception` field with four values

Add an `istio.trafficInterception` field to the Telemetry CR spec with four values: `On`, `PrometheusInputScrapeOnly`, `ExportOnly`, `Off`.

**Pros:**
- Simple, intuitive API with clear semantics
- Single field is easier for users to understand than multiple fields
- Covers all meaningful combinations (everything, scraping-only, export-only, nothing)
- Backward-compatible migration path through phased default change
- Natural progression from `On` → `PrometheusInputScrapeOnly` for Phase 2

**Cons:**
- Global application: All components share the same setting (cannot enable Istio for only Gateway while disabling for Metric Agent)

## Decision

We adopt a single `trafficInterception` field with four values (`On`, `PrometheusInputScrapeOnly`, `ExportOnly`, `Off`). This design balances simplicity and flexibility.

**Rationale:**
- The single-field design is intuitive for users: `On` means "use Istio if available for everything," `PrometheusInputScrapeOnly` means "use Istio only for Metric Agent Prometheus scraping," `ExportOnly` means "use Istio only for export," and `Off` means "never use Istio."
- The four values cover all meaningful combinations while keeping the API simple.
- The two-phase migration provides a backward-compatible transition path that addresses user demand for explicit control (issue [#3549](https://github.com/kyma-project/telemetry-manager/issues/3549)) while preserving existing behavior during the transition.
- Metric Agent Prometheus scraping stays enabled by default in Phase 2 because it is a valuable feature with minimal overhead when Prometheus input is not used or when Istio is not present.

The migration proceeds in two phases: Phase 1 introduces the API with a default of `On` (preserving existing behavior), and Phase 2 changes the default to `PrometheusInputScrapeOnly` (making export opt-in while keeping Metric Agent Prometheus scraping enabled by default).

### API Schema

Add an `istio` field to the Telemetry CR spec with a single trafficInterception control:

```yaml
spec:
   istio:
      trafficInterception: <On | PrometheusInputScrapeOnly | ExportOnly | Off>  # Default: Phase 1: On, Phase 2: PrometheusInputScrapeOnly
```

This design uses a single trafficInterception field that controls Metric Agent Prometheus scraping and export (sending telemetry data to backends) aspects of Istio integration, while applying globally across all telemetry components.

### trafficInterception Semantics

**`On`**: Enable Istio integration for all components (Metric Agent Prometheus scraping, export sidecars for all components) **when Istio is present** (CRDs detected). If Istio is not installed, this mode has no effect - the system applies no Istio configurations and sets no labels.

**`PrometheusInputScrapeOnly`**: Enable Istio integration only for Metric Agent when Prometheus input is enabled to scrape workloads with mTLS (sidecar injection, Istio certificates, mTLS scraping configuration). Disables Istio integration for all other components and for export (no sidecars for sending data to backends). Active when Istio is present. If Istio is not installed, this mode has no effect.

**`ExportOnly`**: Enable Istio integration only for export (sending telemetry data to backends) **when Istio is present**. Disable Istio integration for Metric Agent Prometheus scraping (no mTLS scraping). If the cluster does not have Istio installed, this mode has no effect.

**`Off`**: Explicitly disable Istio integration for all components (no Metric Agent Prometheus scraping, no export sidecars), regardless of whether the cluster has Istio installed. This sets `sidecar.istio.io/inject: "false"` to ensure components do not use Istio even when the cluster has Istio installed.

This makes the API intuitive: `On` means "use Istio if available for everything", `PrometheusInputScrapeOnly` means "use Istio only for Metric Agent Prometheus scraping", `ExportOnly` means "use Istio only for sending data to backends", and `Off` means "never use Istio".

### API Behavior by Component

This section describes how each component behaves under different `istio.trafficInterception` configurations. Each table shows the exact Istio resources, annotations, and behavior for every combination of trafficInterception and Istio presence.

#### OTLP Gateway

The OTLP Gateway receives telemetry data on OTLP ports (4317 gRPC, 4318 HTTP) and forwards it to configured backends.

| trafficInterception         | Sidecar Injection                  | Pod Annotations                                    | Istio Resources                                      |
|-----------------------------|------------------------------------|----------------------------------------------------|------------------------------------------------------|
| `On`                        | `sidecar.istio.io/inject: "true"`  | `traffic.sidecar.istio.io/includeInboundPorts: ""` | DestinationRule (TLS DISABLE), NetworkPolicy (15090) |
| `PrometheusInputScrapeOnly` | `sidecar.istio.io/inject: "false"` | (none)                                             | (none)                                               |
| `ExportOnly`                | `sidecar.istio.io/inject: "true"`  | `traffic.sidecar.istio.io/includeInboundPorts: ""` | DestinationRule (TLS DISABLE), NetworkPolicy (15090) |
| `Off`                       | `sidecar.istio.io/inject: "false"` | (none)                                             | (none)                                               |

#### Metric Agent (Prometheus Input Disabled)

| trafficInterception         | Sidecar Injection                  | Pod Annotations                                                                                                            | Istio Resources       | Prometheus Receivers |
|-----------------------------|------------------------------------|----------------------------------------------------------------------------------------------------------------------------|-----------------------|----------------------|
| `On`                        | `sidecar.istio.io/inject: "true"`  | `traffic.sidecar.istio.io/excludeInboundPorts: "8888"`, `traffic.sidecar.istio.io/includeOutboundPorts: "{backend_ports}"` | NetworkPolicy (15090) | (none)               |
| `PrometheusInputScrapeOnly` | `sidecar.istio.io/inject: "false"` | (none)                                                                                                                     | (none)                | (none)               |
| `ExportOnly`                | `sidecar.istio.io/inject: "true"`  | `traffic.sidecar.istio.io/excludeInboundPorts: "8888"`, `traffic.sidecar.istio.io/includeOutboundPorts: "{backend_ports}"` | NetworkPolicy (15090) | (none)               |
| `Off`                       | `sidecar.istio.io/inject: "false"` | (none)                                                                                                                     | (none)                | (none)               |

#### Metric Agent (Prometheus Input Enabled)

| trafficInterception         | Sidecar Injection                  | Pod Annotations                                                                                                                                                                         | Istio Resources                                           | Prometheus Receivers                                             |
|-----------------------------|------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------|------------------------------------------------------------------|
| `On`                        | `sidecar.istio.io/inject: "true"`  | `traffic.sidecar.istio.io/excludeInboundPorts: "8888"`, `traffic.sidecar.istio.io/includeOutboundPorts: "{backend_ports}"`, `proxy.istio.io/config`, `sidecar.istio.io/userVolumeMount` | NetworkPolicy (15090), Volume (`/etc/istio-output-certs`) | `app-services` (drops HTTPS), `app-services-secure` (HTTPS mTLS) |
| `PrometheusInputScrapeOnly` | `sidecar.istio.io/inject: "true"`  | `traffic.sidecar.istio.io/excludeInboundPorts: "8888"`, `traffic.sidecar.istio.io/includeOutboundPorts: "{backend_ports}"`, `proxy.istio.io/config`, `sidecar.istio.io/userVolumeMount` | NetworkPolicy (15090), Volume (`/etc/istio-output-certs`) | `app-services` (drops HTTPS), `app-services-secure` (HTTPS mTLS) |
| `ExportOnly`                | `sidecar.istio.io/inject: "true"`  | `traffic.sidecar.istio.io/excludeInboundPorts: "8888"`, `traffic.sidecar.istio.io/includeOutboundPorts: "{backend_ports}"`                                                              | NetworkPolicy (15090)                                     | `app-services` (all targets)                                     |
| `Off`                       | `sidecar.istio.io/inject: "false"` | (none)                                                                                                                                                                                  | (none)                                                    | `app-services` (all targets)                                     |

#### Metric Agent (Istio Input Enabled)  

When at least one MetricPipeline has `input.istio.enabled: true`, the Metric Agent scrapes Istio control plane metrics (istiod, Envoy sidecars). This feature is independent of Prometheus input.

**Pod Annotation `traffic.sidecar.istio.io/includeOutboundIPRanges: ""`**:
- **Required when**: `input.istio.enabled: true` and trafficInterception is `On`, `ExportOnly`, or `PrometheusInputScrapeOnly` (sidecar injected)
- **NOT required when**: trafficInterception is `PrometheusInputScrapeOnly` or `Off` (no sidecar, so annotation has no effect, no prometheus input enabled)
- **Purpose**: Bypasses the sidecar to allow direct access to Istio control plane metrics endpoints (istiod, Envoy sidecars)

| trafficInterception         | Sidecar Injection                  | Additional Annotations (for Istio scraping)            |
|-----------------------------|------------------------------------|--------------------------------------------------------|
| `On`                        | `sidecar.istio.io/inject: "true"`  | `traffic.sidecar.istio.io/includeOutboundIPRanges: ""` |
| `PrometheusInputScrapeOnly` | `sidecar.istio.io/inject: "false"` | (none)                                                 |
| `ExportOnly`                | `sidecar.istio.io/inject: "true"`  | `traffic.sidecar.istio.io/includeOutboundIPRanges: ""` |
| `Off`                       | `sidecar.istio.io/inject: "false"` | (none)                                                 |

#### OTel Log Agent

The OTel Log Agent collects container logs using file-based collection and forwards them to backends.

| trafficInterception         |  Sidecar Injection                  | Pod Annotations                                      | Istio Resources       |
|--------------|--------------------------------------|------------------------------------------------------|-----------------------|
| `On`         |  `sidecar.istio.io/inject: "true"`  | `traffic.sidecar.istio.io/excludeInboundPorts: "8888"` | NetworkPolicy (15090) |
| `PrometheusInputScrapeOnly` |  `sidecar.istio.io/inject: "false"` | (none)                                               | (none)                |
| `ExportOnly` |  `sidecar.istio.io/inject: "true"`  | `traffic.sidecar.istio.io/excludeInboundPorts: "8888"` | NetworkPolicy (15090) |
| `Off`        |  `sidecar.istio.io/inject: "false"` | (none)                                               | (none)                |

#### Fluent Bit

Fluent Bit provides legacy log collection capabilities using file-based collection.

| trafficInterception         |  Sidecar Injection                  | Pod Annotations                                              | Istio Resources       |
|--------------|--------------------------------------|--------------------------------------------------------------|-----------------------|
| `On`         |  `sidecar.istio.io/inject: "true"`  | `traffic.sidecar.istio.io/excludeInboundPorts: "2020, 2021"` | NetworkPolicy (15090) |
| `PrometheusInputScrapeOnly` |  `sidecar.istio.io/inject: "false"` | (none)                                                       | (none)                |
| `ExportOnly` |  `sidecar.istio.io/inject: "true"`  | `traffic.sidecar.istio.io/excludeInboundPorts: "2020, 2021"` | NetworkPolicy (15090) |
| `Off`        |  `sidecar.istio.io/inject: "false"` | (none)                                                       | (none)                |


### Istio Processors

**Important**: The `istio_enrichment` and `istio_noise_filter` OTel processors are **independent** from the `trafficInterception` setting. Components configure these processors according to the specifications below, regardless of trafficInterception:

| Component | `istio_enrichment` | `istio_noise_filter` | Notes |
|-----------|-------------------|---------------------|-------|
| OTLP Gateway | Log pipelines only (when OTel log agent is used) | All pipelines (logs, traces, metrics) | Enriches logs with service mesh metadata. Filters noisy Istio telemetry. |
| Metric Agent | Not configured | Metric pipelines | Filters noisy Istio-related metrics. |
| OTel Log Agent | Not configured | Log pipelines | Filters noisy Istio-related logs. |
| Fluent Bit | Not applicable | Not applicable | Fluent Bit does not support OTel processors. |

**Rationale**: These processors operate on telemetry data within the OTel pipeline and do not require Istio certificates or sidecar injection. They provide valuable enrichment and noise reduction regardless of whether Istio integration is enabled for ingestion or export.

### Migration Strategy

The proposed API provides a two-phase migration path. Metric Agent Prometheus scraping capability stays enabled by default while export becomes opt-in. This maintains backward compatibility during the transition.

#### Phase 1: Introduce API with Default trafficInterception On

**Goal**: Provide explicit control while preserving existing behavior

**Changes**:
- We add an `istio.trafficInterception` field with `On | PrometheusInputScrapeOnly | ExportOnly | Off` values
- The default is `On`, which ensures backward compatibility with existing behavior
- Users explicitly set the trafficInterception value to control Metric Agent Prometheus scraping and export independently

**Benefits**:
- **Simple single-field API**: One trafficInterception field controls all Istio integration aspects
- **Clear semantics**: `On` (everything), `PrometheusInputScrapeOnly` (only Metric Agent Prometheus scraping), `ExportOnly` (only export sidecars), `Off` (nothing)
- **Flexible control**: Users can enable Istio for Metric Agent Prometheus scraping only, export only, both, or neither
- **Intelligent traffic routing**: When export is enabled and Istio is present, the system analyzes pipeline URLs to configure traffic routing annotations (cluster-internal URLs route through sidecar, external URLs bypass)
- **Backward compatibility**: Default `On` ensures existing behavior is preserved
- **Natural progression**: Clear path from `On` → `PrometheusInputScrapeOnly` for Phase 2 migration

**Limitations**:
- **Global application**: All components share the same trafficInterception (cannot enable Istio for only Gateway while disabling for Metric Agent)
- **URL-based heuristic**: Cluster-internal URL detection might not capture all cases (for example, ServiceEntry-backed services)

**Migration for Existing Clusters**:

For clusters with Istio:
- **No action needed**: The default `trafficInterception: On` preserves current behavior
- **Optional optimization**: Users can set `trafficInterception: PrometheusInputScrapeOnly` to save sidecar overhead if backends are external, or `trafficInterception: Off` to disable Istio entirely

For clusters without Istio:
- **No action needed**: Detection logic checks for Istio CRDs before applying configurations
- **Optional explicit configuration**: Users can set `trafficInterception: Off` to document intent

#### Phase 2: Change Default trafficInterception to PrometheusInputScrapeOnly

**Goal**: Make export Istio mode opt-in by default while keeping Metric Agent Prometheus scraping enabled (addresses issue [#657](https://github.com/kyma-project/telemetry-manager/issues/657))

**Changes**:
- We change the `istio.trafficInterception` default from `On` to `PrometheusInputScrapeOnly` (keeps Metric Agent Prometheus scraping enabled, makes export opt-in)
- Users who need Istio integration for export explicitly set `trafficInterception: On` in their Telemetry CR

**Rationale for Keeping Metric Agent Prometheus Scraping Enabled (PrometheusInputScrapeOnly)**:
- **Metric Agent Prometheus scraping functionality**: The Metric Agent's ability to scrape workloads with Istio STRICT mTLS policies using Prometheus input is a valuable feature that users expect to work by default when Istio is present
- **Minimal overhead when Istio not present**: This mode only activates when Istio CRDs are detected, so there's no cost in non-Istio clusters
- **Minimal overhead when Prometheus input not used**: Only the Metric Agent gets sidecar injection when Prometheus input is enabled; all other components have no sidecars and minimal Istio configuration
- **User expectations**: Prometheus scraping of mTLS workloads is expected to "just work" when both Istio and Telemetry Manager are installed

**Rationale for Disabling Export by Default**:
- **Resource efficiency**: Export mode injects sidecars into all telemetry components, adding significant resource overhead
- **Explicit opt-in for mesh integration**: Users should consciously decide when to integrate telemetry components into the Istio mesh for export
- **Most common use case**: Many users send telemetry to external backends that don't require Istio mTLS

**Migration Strategy**:

1. **Announce change**: Provide advance notice (at least 2 releases) that the default trafficInterception will change to `PrometheusInputScrapeOnly`
2. **Audit tooling**: Provide a command or script that checks existing clusters for:
   - Istio presence
   - Pipelines with cluster-internal output URLs
   - Generates recommended Telemetry CR configurations
3. **Documentation**: Update all examples and guides to show explicit trafficInterception configuration

**Phase 2 Benefits**:
- **Resource efficiency for common case**: Most users send to external backends and won't pay sidecar overhead
- **Explicit export configuration**: Users consciously decide when to integrate with Istio mesh for export
- **Cleaner defaults**: New clusters start with minimal overhead for export and keep Metric Agent Prometheus scraping functionality enabled

**Phase 2 Breaking Changes**:
- Existing Telemetry CRs without explicit `istio.trafficInterception` configuration will have export Istio disabled after upgrade

**Timeline**:
- **Phase 1**: Immediate implementation (`trafficInterception: On` default)
- **Phase 2**: After Phase 1 has been stable (`trafficInterception: PrometheusInputScrapeOnly` default)

## Consequences

### Positive Consequences

- **Explicit control**: Users gain explicit control over Istio integration (addresses issue [#3549](https://github.com/kyma-project/telemetry-manager/issues/3549)), allowing them to disable Istio mode when not needed
- **Backward compatibility**: The two-phase migration preserves existing behavior during Phase 1, preventing unexpected breakage
- **Resource efficiency**: Phase 2 reduces resource overhead for users who send telemetry to external backends and don't need export Istio mode
- **Clear semantics**: The four values (`On`, `PrometheusInputScrapeOnly`, `ExportOnly`, `Off`) provide intuitive, self-documenting configuration
- **Preserved functionality**: Metric Agent Prometheus scraping remains enabled by default in Phase 2, preserving the ability to scrape mTLS workloads without user intervention
- **Flexible migration path**: Users can optimize their configuration before Phase 2 by explicitly setting `PrometheusInputScrapeOnly` to save sidecar overhead if backends are external

### Negative Consequences

- **Global application**: All components share the same `trafficInterception` setting. Users cannot selectively enable Istio for specific components (for example, enable for Gateway but disable for Metric Agent). This limitation is acceptable because most users need consistent Istio integration across all components.
- **URL-based heuristic**: The system infers cluster-internal backends from URLs to configure traffic routing annotations, which might not capture all cases (for example, cluster-external URLs routed internally via DNS or ServiceEntry-backed services). Users must understand this limitation when configuring export backends.
- **Phase 2 breaking change**: Users who rely on implicit export Istio mode must explicitly set `trafficInterception: On` after the Phase 2 upgrade. Migration tooling and advance notice (at least 2 releases) mitigate this risk.
- **Migration complexity**: The two-phase approach requires careful communication and documentation to ensure users understand the timeline and required actions.

### Removed Istio Resources

The following Istio resources are no longer needed and have been removed from the OTLP Gateway configuration:

#### PeerAuthentication Resource

**Previous behavior**: The Gateway applied a PeerAuthentication resource with PERMISSIVE mTLS mode when Istio was detected.

**Why it's no longer needed**: The `traffic.sidecar.istio.io/includeInboundPorts: ""` annotation already bypasses all inbound traffic interception by the sidecar. Because the sidecar never intercepts inbound OTLP traffic (ports 4317, 4318), the PeerAuthentication resource has no effect. The Gateway receives all telemetry data directly without mTLS enforcement, making the explicit PERMISSIVE policy redundant.

**Impact**: Removing this resource simplifies the Gateway configuration without changing behavior. Direct connections to OTLP ports remain plain-text, and the Gateway continues to accept telemetry from both in-mesh and out-of-mesh clients.

#### Pod Annotation: `sidecar.istio.io/interceptionMode: TPROXY`

**Previous behavior**: The Gateway pod had the `sidecar.istio.io/interceptionMode: TPROXY` annotation when Istio was detected.

**Why it's no longer needed**: This annotation only affects traffic that the sidecar intercepts. However, `traffic.sidecar.istio.io/includeInboundPorts: ""` disables all inbound interception, so the sidecar never processes inbound OTLP traffic. The TPROXY mode setting has no operational impact because there is no intercepted inbound traffic to apply it to.

**Impact**: Removing this annotation has no behavioral effect. The annotation was ineffective in the current configuration because inbound interception was already disabled.
