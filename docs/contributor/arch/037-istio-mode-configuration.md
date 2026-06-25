---
title: Istio Mode Configuration API
status: Proposed
date: 2026-06-10
---

# Istio Mode Configuration API

## Context and Problem Statement

Telemetry Manager currently auto-detects Istio by checking for Istio CRDs (`*.istio.io`) in the cluster and automatically applies Istio-specific resources when detected. This includes sidecar injection labels, DestinationRules, traffic routing annotations, and certificate volume mounts across telemetry components (OTLP Gateway, Metric Agent, OTel Log Agent, Fluent Bit, and Self-Monitor).

Although automatic detection provides convenience, several operational challenges exist:

- No explicit control: Users have no way to disable Istio mode when they don't need it. This causes sidecar containers, certificate management, and Istio-specific network policies to create unnecessary resource overhead.

- Unconditional artifacts: Telemetry Manager applies some Istio-related behaviors unconditionally, regardless of whether it detects Istio. For example, all components (except Self-Monitor) always have `sidecar.istio.io/inject: "true"` and the Metric Agent always mounts Istio certificate volumes, even when Istio integration is not required.

- All-or-nothing approach: The current design does not allow granular control. Users cannot selectively enable Istio mode for specific components or use cases (such as backends requiring in-cluster mTLS).

- Migration difficulty: To move toward an "Istio mode OFF by default" model (tracked in [issue #657](https://github.com/kyma-project/telemetry-manager/issues/657)), the system requires a backward-compatible transition path that preserves existing behavior while allowing explicit opt-in configuration.

The system requires an API mechanism to explicitly enable or disable Istio mode, providing users with control over when and how Istio integration is applied to telemetry components. This addresses [issue #3549](https://github.com/kyma-project/telemetry-manager/issues/3549), which proposes an explicit configuration model that supports eventual migration to an opt-in default while maintaining backward compatibility during the transition.

## Current Istio Integration

### Istio Detection

The Telemetry Manager automatically detects Istio by checking for the presence of Istio CRDs (Custom Resource Definitions) in the cluster. Specifically, it looks for API groups matching `*.istio.io`.

### Component Behaviors

#### OTLP Gateway

The OTLP Gateway is deployed as a DaemonSet and serves as the unified ingress point for all telemetry signals (logs, traces, and metrics).

##### Unconditional (Always Applied)

| Resource  | Behavior                          | Rationale                                                                                                                                       |
|-----------|-----------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Label | `sidecar.istio.io/inject: "true"` | The Gateway forwards telemetry data to backends. When backends are in-cluster services within the Istio mesh with STRICT mTLS policies, the Gateway needs the sidecar to establish mTLS connections for output. |

##### Conditional (Only When Istio is Detected)

| Resource                | Behavior                                                          | Rationale                                                                                                                                                                                                                                                                                          |
|-------------------------|-------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Annotation          | `sidecar.istio.io/interceptionMode: TPROXY`                       | This annotation is ineffective in the current configuration because `includeInboundPorts: ""` disables all inbound interception. TPROXY only affects traffic that is intercepted by the sidecar.                                                                                                   |
| Pod Annotation          | `traffic.sidecar.istio.io/includeInboundPorts: ""`                | The Gateway receives telemetry data on OTLP ports (4317, 4318) from both in-mesh and out-of-mesh clients. Excluding all inbound ports from sidecar interception allows direct connections without requiring mTLS, improving ingestion performance and compatibility.                               |
| PeerAuthentication      | PERMISSIVE mTLS mode                                              | The Gateway must accept telemetry from clients that cannot provide mTLS certificates (out-of-mesh workloads, legacy clients). PERMISSIVE mode allows both plain-text and mTLS connections, supporting the Gateway's role as a universal ingress point.                                             |
| DestinationRule         | `TLS mode: DISABLE` for all OTLP Services                         | The Gateway Services (OTLP ingress endpoints) use node-local routing where clients connect to the same-node Gateway pod without encryption. Other telemetry components and application workloads must reach these endpoints over plain-text, so TLS is disabled for client-to-Gateway connections. |
| NetworkPolicy (Ingress) | Additionally permits traffic on Istio Envoy telemetry port (15090) | The Gateway's sidecar exposes Envoy metrics on port 15090. The monitoring systems need to scrape these metrics to observe the Gateway's sidecar metrics and health.                                                                                                                                |

#### Metric Agent

The Metric Agent is deployed as a DaemonSet and scrapes Prometheus metrics from workloads in the cluster, then forwards them to configured backends.

##### Unconditional (Always Applied)

| Resource                 | Behavior                                                                                                              | Rationale                                                                                                                                                                                 |
|--------------------------|-----------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Label                | `sidecar.istio.io/inject: "true"`                                                                                     | The Metric Agent forwards scraped metrics to backends. When backends are in-cluster services within the Istio mesh with STRICT mTLS policies, the Agent needs the sidecar to establish mTLS connections for output.                                           |
| Prometheus Scrape Config | `app-services` job scrapes services with `prometheus.io/scrape: "true"` annotation | The Metric Agent must scrape application metrics from workloads that expose plain-text Prometheus endpoints. This job handles non-mTLS targets by dropping HTTPS endpoints (which require certificates the Agent doesn't have when Istio input is disabled). |

##### Conditional (Only When Istio is Detected and Prometheus Input Enabled)

| Resource                            | Behavior                                                                                         | Rationale                                                                                                                                                                                                                                                             |
|-------------------------------------|--------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Annotation                      | `traffic.sidecar.istio.io/includeOutboundIPRanges: ""`                                           | The Metric Agent scrapes Istio control plane components (istiod) and workload Envoy sidecars directly on their metric endpoints. These endpoints are not accessible through the sidecar proxy, so most outbound traffic must bypass the sidecar for scraping to work.                                    |
| Pod Annotation                      | `traffic.sidecar.istio.io/includeOutboundPorts: "{backend_ports}"`                               | The Metric Agent forwards scraped metrics to configured backends. When backends are in-cluster services in the Istio mesh, traffic to their ports must route through the sidecar for mTLS. The Agent dynamically populates this annotation with backend ports from MetricPipeline configurations. |
| Pod Annotation                      | `traffic.sidecar.istio.io/excludeInboundPorts: "8888"`                                           | The Metric Agent exposes its own Prometheus metrics on port 8888. Monitoring systems (Self-Monitor) need to scrape these metrics directly without requiring mTLS, so this port is excluded from sidecar interception.                                                                                                 |
| Pod Annotation                      | `proxy.istio.io/config`                                                                          | The Metric Agent needs Istio certificates to scrape application workloads with STRICT mTLS policies. This annotation configures the sidecar to write certificates to a shared volume at `/etc/istio-output-certs` where the Agent can read them.                                                                                       |
| Pod Annotation                      | `sidecar.istio.io/userVolumeMount`                                                               | This annotation mounts the Istio certificate volume into the sidecar container so the sidecar can write certificates to the shared location.                                                                                                                                                                                                 |
| Prometheus Scrape Config            | `app-services-secure` job scrapes HTTPS targets with TLS config pointing to `/etc/istio-output-certs`                  | The Metric Agent must scrape application workloads with STRICT mTLS policies that require client certificates. This job uses the Istio certificates from the sidecar to authenticate when scraping these protected endpoints, keeping only HTTPS targets.                                            |
| Volume Mount                        | Istio certificates volume (`/etc/istio-output-certs`)                                            | The Metric Agent needs access to Istio-provided mTLS certificates to scrape workloads with STRICT mTLS policies. The system mounts this shared volume when Istio is detected and Prometheus input is enabled. |
| NetworkPolicy (Ingress)             | Additionally permits traffic on Istio Envoy telemetry port (15090)                                | The Metric Agent's sidecar exposes Envoy metrics on port 15090. Monitoring systems need to scrape these metrics to observe the Agent's outbound mTLS connections and sidecar health.                                                                                                                                                                 |

**Key Behavior**: The `app-services` and `app-services-secure` jobs work together to provide complete coverage:
- `app-services`: Always configured. Scrapes workloads that expose plain-text Prometheus metrics (drops `https` targets).
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
| NetworkPolicy (Ingress) | Additionally permits traffic on Istio Envoy telemetry port (15090) | The OTel Log Agent's sidecar exposes Envoy metrics on port 15090. Monitoring systems need to scrape these metrics to observe the Agent's outbound mTLS connections and sidecar health. |

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
| NetworkPolicy (Ingress) | Additionally permits traffic on Istio Envoy telemetry port (15090) | Fluent Bit's sidecar exposes Envoy metrics on port 15090. Monitoring systems need to scrape these metrics to observe Fluent Bit's outbound mTLS connections and sidecar health.                                                          |


#### Self-Monitor

The Self-Monitor is a Prometheus instance deployed as a Deployment that scrapes metrics from Telemetry components for health monitoring and alerting.

##### Unconditional (Always Applied)

| Resource  | Behavior                           | Rationale                                                                                                                                                                                                                           |
|-----------|------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Label | `sidecar.istio.io/inject: "false"` | The Self-Monitor scrapes metrics only from Telemetry components within the same namespace (kyma-system) over plain-text connections. These targets do not require mTLS authentication. Disabling sidecar injection reduces resource overhead since the Self-Monitor never needs to communicate with in-mesh services. |


##### Conditional (Only When Istio is Detected)

The system creates no Istio-specific resources for the Self-Monitor because it explicitly disables sidecar injection.

### Summary

| Component      | Sidecar Injection | Istio Certificates | Special Annotations           | Istio-Specific Resources       |
|----------------|-------------------|--------------------|-------------------------------|--------------------------------|
| OTLP Gateway   | Always enabled    | Not used           | None                          | DestinationRule (TLS DISABLE)  |
| Metric Agent   | Always enabled    | Always mounted     | Conditional (traffic routing) | None                           |
| OTel Log Agent | Always enabled    | Not used           | None                          | None                           |
| Fluent Bit     | Always enabled    | Not used           | None                          | None                           |
| Self-Monitor   | Always disabled   | Not used           | None                          | None                           |

## Proposed Solution

### API Schema

Add an `istio` field to the Telemetry CR spec with a single mode control:

```yaml
spec:
  istio:
    mode: <On | IngestOnly | ExportOnly | Off>  # Default: Phase 1: On, Phase 2: IngestOnly
```

This design uses a single mode field that controls both ingestion (receiving or scraping telemetry data) and export (sending telemetry data to backends) aspects of Istio integration, while applying globally across all telemetry components.

### Mode Semantics

**`On`**: Enable Istio integration for both ingestion and export **when Istio is present** (CRDs detected). If Istio is not installed, this mode has no effect - no Istio configurations are applied, and no labels are set.

**`IngestOnly`**: Enable Istio integration only for ingestion (Gateway receiving data, Metric Agent scraping mTLS workloads) **when Istio is present**. Disable Istio integration for export (no sidecars for sending data to backends). If Istio is not installed, this mode has no effect.

**`ExportOnly`**: Enable Istio integration only for export (sending telemetry data to backends) **when Istio is present**. Disable Istio integration for ingestion (no DestinationRules, no mTLS scraping). If Istio is not installed, this mode has no effect.

**`Off`**: Explicitly disable Istio integration for both ingestion and export, regardless of whether Istio is present. This sets `sidecar.istio.io/inject: "false"` to ensure components do not use Istio even if it's installed.

This makes the API intuitive: `On` means "use Istio if available for everything", `IngestOnly` means "use Istio only for receiving/scraping data", `ExportOnly` means "use Istio only for sending data to backends", and `Off` means "never use Istio".

### Mode: On

Enable Istio integration for both ingestion and export **when Istio is present**.

### Mode: On

Enable Istio integration for both ingestion and export **when Istio is present**:

#### Ingestion Behavior

**Gateway**: 
- If Istio is present, the system applies DestinationRule (TLS DISABLE) for OTLP services and enables sidecar injection with traffic routing annotations.
- If Istio is not present, the system applies no Istio configurations and sets no labels.

**Metric Agent**: 
- If Istio is present and at least one MetricPipeline has `input.prometheus.enabled: true`, the system mounts Istio certificates, configures the `app-services-secure` Prometheus scrape job, and enables sidecar injection.
- If Istio is present but no MetricPipeline has Prometheus input enabled, the system sets `sidecar.istio.io/inject: "false"` (Istio available but not needed for this component).
- If Istio is not present, the system applies no Istio configurations and sets no labels.

**Log Agents**: 
- No effect (file-based log collection does not require Istio for ingestion)

**Metric Agent Receiver Configuration**:

| Prometheus Input | Istio Present | Receiver Configuration                                                                                   |
|------------------|---------------|----------------------------------------------------------------------------------------------------------|
| Enabled          | Yes           | `app-services` (drops HTTPS targets) + `app-services-secure` (scrapes STRICT mTLS workloads with certs) |
| Enabled          | No            | `app-services` (scrapes all targets including HTTPS)                                                     |
| Disabled         | Any           | No Prometheus receivers configured                                                                       |

#### Export Behavior

**All Components**: 
- If Istio is present, the system enables sidecar injection for all telemetry components.
- If Istio is not present, the system applies no Istio configurations and sets no labels.

**Traffic Routing Configuration** (when Istio present):
- The system analyzes each pipeline's output URL to determine which ports need traffic routing through the sidecar
- **Cluster-internal URLs** (for example, `http://otel-collector.observability.svc.cluster.local:4317`): Backend ports are added to `traffic.sidecar.istio.io/includeOutboundPorts` annotation to enable mTLS communication
- **External URLs** (for example, `https://logs.external.com`): Backend ports bypass the sidecar (direct connection, no mTLS)

### Mode: IngestOnly

Enable Istio integration only for ingestion, disable for export **when Istio is present**:

#### Ingestion Behavior

Same as Mode: On (see above).

#### Export Behavior

**All Components**: 
- The system sets `sidecar.istio.io/inject: "false"`.
- Components cannot send data to STRICT mTLS backends in the Istio mesh, even if they use cluster-internal URLs.

### Mode: ExportOnly

Enable Istio integration only for export, disable for ingestion **when Istio is present**:

#### Ingestion Behavior

**Gateway**: 
- The system creates no DestinationRule and sets `sidecar.istio.io/inject: "true"` (sidecar enabled for export only).

**Metric Agent**: 
- The system mounts no Istio certificates and configures no `app-services-secure` receiver.
- The Metric Agent cannot scrape STRICT mTLS workloads.
- The `app-services` receiver scrapes all targets including HTTPS (will fail for STRICT mTLS workloads).
- The system sets `sidecar.istio.io/inject: "true"` (sidecar enabled for export only).

**Log Agents**: 
- The system sets `sidecar.istio.io/inject: "true"` (sidecar enabled for export only).

**Metric Agent Receiver Configuration**:

| Prometheus Input | Receiver Configuration                               |
|------------------|------------------------------------------------------|
| Enabled          | `app-services` (scrapes all targets including HTTPS) |
| Disabled         | No Prometheus receivers configured                   |

#### Export Behavior

Same as Mode: On (see above).

### Mode: Off

Explicitly disable Istio integration for both ingestion and export:

#### Ingestion Behavior

**Gateway**: 
- The system creates no DestinationRule and sets `sidecar.istio.io/inject: "false"`.

**Metric Agent**: 
- The system mounts no Istio certificates and configures no `app-services-secure` receiver.
- The Metric Agent cannot scrape STRICT mTLS workloads.
- The `app-services` receiver scrapes all targets including HTTPS (will fail for STRICT mTLS workloads).
- The system sets `sidecar.istio.io/inject: "false"`.

**Log Agents**: 
- The system sets `sidecar.istio.io/inject: "false"`.

**Metric Agent Receiver Configuration**:

| Prometheus Input | Receiver Configuration                               |
|------------------|------------------------------------------------------|
| Enabled          | `app-services` (scrapes all targets including HTTPS) |
| Disabled         | No Prometheus receivers configured                   |

#### Export Behavior

**All Components**: 
- The system sets `sidecar.istio.io/inject: "false"`.
- Components cannot send data to STRICT mTLS backends in the Istio mesh, even if they use cluster-internal URLs.

### Istio Processors

**Important**: The `istio_enrichment` and `istio_noise_filter` OTel processors are **independent** from the `mode` setting. OTel workloads include these processors in their pipelines according to the specifications below, regardless of Istio mode configuration.

**Configuration per component**:

1. **OTLP Gateway**:
   - Configure `istio_enrichment` processor **only in log pipelines** when the log pipeline is active (not present when using legacy Fluent Bit for logs).
   - Always configure `istio_noise_filter` processor in trace, metric, and log pipelines.
   - Independent of `mode` setting.
   - Enriches logs with Istio service mesh metadata (workload identity, mesh attributes).
   - Filters out noisy Istio-related telemetry (health checks, internal communication).

2. **Metric Agent**:
   - Does not configure `istio_enrichment` processor.
   - Always configure `istio_noise_filter` processor in metric pipelines.
   - Independent of `mode` setting.
   - Filters out noisy Istio-related metrics.

3. **OTel Log Agent**:
   - Does not configure `istio_enrichment` processor.
   - Always configure `istio_noise_filter` processor in log pipelines.
   - Independent of `mode` setting.
   - Filters out noisy Istio-related logs.

4. **Fluent Bit**:
   - Fluent Bit does not support OTel processors, so neither `istio_enrichment` nor `istio_noise_filter` are applicable.

**Rationale**: The `istio_noise_filter` processor provides valuable noise reduction for all signal types. The `istio_enrichment` processor is used by the Gateway to enrich trace, metric, and log data with service mesh context. These processors operate on telemetry data within the OTel pipeline and do not require Istio certificates or sidecar injection.

### Migration Path

The proposed API provides a two-phase migration path where ingestion remains stable while export transitions to opt-in, maintaining backward compatibility during the transition.

#### Phase 1: Introduce API with Default Mode On

**Goal**: Provide explicit control while preserving existing behavior

**Changes**:
- Add `istio.mode` field with `On | IngestOnly | ExportOnly | Off` values
- Default to `On` to ensure backward compatibility with existing behavior
- Users can explicitly change the mode to control ingestion and/or export independently

**Benefits**:
- **Simple single-field API**: One mode field controls all Istio integration aspects
- **Clear semantics**: `On` (everything), `IngestOnly` (receiving/scraping only), `ExportOnly` (sending only), `Off` (nothing)
- **Flexible control**: Users can enable Istio for ingestion only, export only, both, or neither
- **Intelligent traffic routing**: When export is enabled and Istio is present, the system analyzes pipeline URLs to configure traffic routing annotations (cluster-internal URLs route through sidecar, external URLs bypass)
- **Backward compatibility**: Default `On` ensures existing behavior is preserved
- **Natural progression**: Clear path from `On` → `IngestOnly` for Phase 2 migration

**Limitations**:
- **Global application**: All components share the same mode (cannot enable Istio for only Gateway while disabling for Metric Agent)
- **URL-based heuristic**: Cluster-internal URL detection might not capture all cases (for example, ServiceEntry-backed services)

**Migration for Existing Clusters**:

For clusters with Istio:
- **No action needed**: The default `mode: On` preserves current behavior
- **Optional optimization**: Users can set `mode: IngestOnly` to save sidecar overhead if backends are external, or `mode: Off` to disable Istio entirely

For clusters without Istio:
- **No action needed**: Detection logic checks for Istio CRDs before applying configurations
- **Optional explicit configuration**: Users can set `mode: Off` to document intent

#### Phase 2: Change Default Mode to IngestOnly

**Goal**: Make export Istio mode opt-in by default while keeping ingestion enabled (addresses [issue #657](https://github.com/kyma-project/telemetry-manager/issues/657))

**Changes**:
- Change `istio.mode` default from `On` to `IngestOnly` (ingestion remains enabled, export becomes opt-in)
- Users who need Istio integration for export must explicitly set `mode: On` in their Telemetry CR

**Rationale for Keeping Ingestion Enabled (IngestOnly)**:
- **Metric Agent ingestion functionality**: The Metric Agent's ability to scrape STRICT mTLS workloads using Prometheus input is a valuable feature that users expect to work by default when Istio is present
- **Gateway ingestion requirements**: The Gateway requires DestinationRule configuration to receive telemetry data correctly in Istio meshes
- **Minimal overhead when Istio not present**: Ingestion mode only activates when Istio CRDs are detected, so there's no cost in non-Istio clusters
- **User expectations**: Ingestion features (scraping mTLS workloads, receiving data in mesh) are expected to "just work" when both Istio and Telemetry Manager are installed

**Rationale for Disabling Export by Default**:
- **Resource efficiency**: Export mode injects sidecars into all telemetry components, adding significant resource overhead
- **Explicit opt-in for mesh integration**: Users should consciously decide when to integrate telemetry components into the Istio mesh for export
- **Most common use case**: Many users send telemetry to external backends that don't require Istio mTLS

**Migration Strategy**:

1. **Announce change**: Provide advance notice (at least 2 releases) that the default mode will change to `IngestOnly`
2. **Audit tooling**: Provide a command or script that checks existing clusters for:
   - Istio presence
   - Pipelines with cluster-internal output URLs
   - Generates recommended Telemetry CR configurations
3. **Documentation**: Update all examples and guides to show explicit mode configuration

**Phase 2 Benefits**:
- **Resource efficiency for common case**: Most users send to external backends and won't pay sidecar overhead
- **Explicit export configuration**: Users consciously decide when to integrate with Istio mesh for export
- **Cleaner defaults**: New clusters start with minimal overhead for export while maintaining ingestion functionality

**Phase 2 Breaking Changes**:
- Existing Telemetry CRs without explicit `istio.mode` configuration will have export Istio disabled after upgrade

**Timeline**:
- **Phase 1**: Immediate implementation (`mode: On` default)
- **Phase 2**: After Phase 1 has been stable for at least 2 releases (`mode: IngestOnly` default)
**Timeline**:
- **Phase 1**: Immediate implementation (`mode: On` default)
- **Phase 2**: After Phase 1 has been stable for at least 2 releases (`mode: IngestOnly` default)

### Backward Compatibility

#### Phase 1: Default Mode On

The single-mode API approach ensures backward compatibility during Phase 1:

- **Default `On` mode**: Preserves current behavior where Istio integration is enabled for both ingestion and export when Istio CRDs are present
- **No breaking changes**: Existing Telemetry CRs without the `istio` field work with the default `On` mode, which matches current implicit behavior
- **Explicit control when needed**: Users can set `mode: IngestOnly`, `mode: ExportOnly`, or `mode: Off` to control Istio integration as needed

**Comparison with Current Behavior**:

| Current (Implicit)                   | Phase 1 API Equivalent | Behavior                                                                        |
|--------------------------------------|------------------------|---------------------------------------------------------------------------------|
| Always enabled (when Istio detected) | `mode: On` (default)   | Full Istio integration for ingestion and export when Istio is present          |
| No explicit disable option           | `mode: Off`            | All Istio disabled, opt-out for resource savings                                |
| N/A                                  | `mode: IngestOnly`     | Istio enabled only for ingestion (Gateway DestinationRule, mTLS scraping)      |
| N/A                                  | `mode: ExportOnly`     | Istio enabled only for export (sidecars for sending to in-mesh backends)        |

**Migration for Existing Clusters**:

For clusters with Istio:
- **No action needed**: The default `mode: On` preserves current behavior
- **Optional optimization**: Users can set `mode: IngestOnly` to save sidecar overhead if backends are external, or `mode: Off` if they don't need Istio integration at all

For clusters without Istio:
- **No action needed**: Detection logic checks for Istio CRDs before applying configurations
- **Optional explicit configuration**: Users can set `mode: Off` to document intent

#### Phase 2: Default Mode IngestOnly

When Phase 2 is implemented, the backward compatibility approach changes:

- **Default `IngestOnly` mode**: Ingestion Istio integration remains enabled by default when Istio CRDs are present; export becomes opt-in
- **Explicit opt-in for export**: Users must explicitly set `mode: On` to enable Istio sidecar injection for export traffic

**Comparison with Phase 1 Behavior**:

| Scenario                             | Phase 1 (Mode On Default)   | Phase 2 (Mode IngestOnly Default)    |
|--------------------------------------|----------------------------|--------------------------------------|
| New Telemetry CR, no `istio` field   | `mode: On` (default)       | `mode: IngestOnly` (default)         |
| Existing Telemetry CR during upgrade | No change                  | User must opt in with `mode: On`     |
| Explicit `istio: {mode: On}`         | Behavior unchanged         | Behavior unchanged                   |
| User wants ingestion only            | Must set `mode: IngestOnly`| No action needed (default behavior)  |
