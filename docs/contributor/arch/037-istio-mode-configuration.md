---
title: Istio Mode Configuration API
status: Proposed
date: 2026-06-10
---

# Istio Mode Configuration API

## Context and Problem Statement

Telemetry Manager currently auto-detects Istio by checking for Istio CRDs (`*.istio.io`) in the cluster and automatically applies Istio-specific resources when detected. This includes sidecar injection labels, DestinationRules, traffic routing annotations, and certificate volume mounts across telemetry components (OTLP Gateway, Metric Agent, OTel Log Agent, Fluent Bit, and Self-Monitor).

Although automatic detection provides convenience, several operational challenges exist:

- No explicit control: Because users cannot disable Istio mode even when not needed, sidecar containers, certificate management, and Istio-specific network policies create unnecessary resource overhead.

- Unconditional artifacts: The system applies some Istio-related behaviors unconditionally regardless of detection. For example, all components (except Self-Monitor) always have `sidecar.istio.io/inject: "true"` and the Metric Agent always mounts Istio certificate volumes, even when Istio integration is not required.

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
| Pod Label | `sidecar.istio.io/inject: "true"` | Istio sidecar injection is always enabled for the OTLP Gateway to support output mTLS communication to in-cluster backends in the Istio mesh. |

##### Conditional (Only When Istio is Detected)

| Resource                | Behavior                                                          | Rationale                                                                                                                                                                                                                                                     |
|-------------------------|-------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Annotation          | `sidecar.istio.io/interceptionMode: TPROXY`                       | Configures the Istio sidecar to use transparent proxy mode for traffic interception.                                                                                                                                                                          |
| Pod Annotation          | `traffic.sidecar.istio.io/includeInboundPorts: ""`                | This annotation excludes all input ports from Istio sidecar interception, ensuring direct access to OTLP ingestion endpoints without mTLS overhead. |
| PeerAuthentication      | PERMISSIVE mTLS mode                                              | This configuration supports both plain-text and mTLS connections to the OTLP Gateway. Applications can send data over plain-text because the ingestion is node-local and can use mTLS communication. |
| DestinationRule         | `TLS mode: DISABLE` for all OTLP Services                         | Because other components must connect without requiring mTLS, the OTLP Gateway receives telemetry data over plain-text on the ingestion path because of node-local routing. Disabling TLS for client connections to these services supports this requirement. |
| NetworkPolicy (Ingress) | Additionally permits traffic on Istio Envoy telemetry port (15090) | When Istio is present, the sidecar's Envoy proxy exposes metrics that monitoring systems must scrape.                                                                                                                                                         |

#### Metric Agent

The Metric Agent is deployed as a DaemonSet and scrapes Prometheus metrics from workloads in the cluster, then forwards them to configured backends.

##### Unconditional (Always Applied)

| Resource                     | Behavior                                                                                                              | Rationale                                                                                                                                                                                 |
|------------------------------|-----------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Label                    | `sidecar.istio.io/inject: "true"`                                                                                     | Istio sidecar injection is always enabled for the Metric Agent to support output mTLS communication to in-cluster backends in the Istio mesh.                                           |
| Prometheus Scrape Config     | `app-services` job with relabel config to drop pods with `security.istio.io/tlsMode: istio` label when scheme is `https` | Always configured. Scrapes application services with `prometheus.io/scrape: "true"` annotation. Drops HTTPS targets (STRICT mTLS workloads) to avoid scraping them without certificates. |
| `app-services` relabel rules | Drops targets with `__scheme__: https`                                                                                | Prevents scraping STRICT mTLS workloads without proper certificates if Istio certificates are not mounted. |

##### Conditional (Only When Istio is Detected and Prometheus Input Enabled)

| Resource                            | Behavior                                                                                         | Rationale                                                                                                                                                                                                                                                             |
|-------------------------------------|--------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Annotation                      | `traffic.sidecar.istio.io/includeOutboundIPRanges: ""`                                           | Bypasses Istio sidecar interception for most output traffic. Prometheus scraping of Istio control plane and Envoy metrics requires direct access to metric endpoints. These endpoints are not reachable through the sidecar proxy.                                    |
| Pod Annotation                      | `traffic.sidecar.istio.io/includeOutboundPorts: "{backend_ports}"`                               | Ensures that traffic to configured backends (such as OTLP Gateway, in-cluster Prometheus, or other OTel Collectors) goes through the Istio sidecar for mTLS. The reconciliation loop populates this with the actual backend ports from MetricPipeline configurations. |
| Pod Annotation                      | `traffic.sidecar.istio.io/excludeInboundPorts: "8888"`                                           | Excludes the metrics port from Istio sidecar interception, ensuring that monitoring systems can scrape the Metric Agent's own metrics directly without mTLS overhead.                                                                                                 |
| Pod Annotation                      | `proxy.istio.io/config`                                                                          | Configures the Istio sidecar to write TLS certificates to the shared volume at `/etc/istio-output-certs`, which the Metric Agent uses for mTLS scraping of application metrics.                                                                                       |
| Pod Annotation                      | `sidecar.istio.io/userVolumeMount`                                                               | Mounts the Istio certificate volume into the Istio sidecar container.                                                                                                                                                                                                 |
| Prometheus Scrape Config            | `app-services-secure` job with TLS config pointing to `/etc/istio-output-certs`                  | Scrapes application services with STRICT mTLS policies using Istio certificates. Only targets with `__scheme__: https` are kept. The Metric Agent can scrape workloads in the Istio mesh that require mTLS authentication.                                            |
| `app-services-secure` TLS           | `ca_file`, `cert_file`, `key_file` from `/etc/istio-output-certs/`, `insecure_skip_verify: true` | Uses Istio-provided certificates for mTLS authentication when scraping STRICT mTLS workloads.                                                                                                                                                                         |
| `app-services-secure` relabel rules | Keeps only targets with `__scheme__: https` (opposite of `app-services`)                         | Ensures this job only scrapes STRICT mTLS workloads, complementing the `app-services` job which handles non-mTLS targets.                                                                                                                                             |
| NetworkPolicy (Ingress)             | Additionally permits traffic on Istio Envoy telemetry port (15090)                                | When Istio is present, the sidecar's Envoy proxy exposes metrics that monitoring systems must scrape.                                                                                                                                                                 |
| Volume Mount                        | Istio certificates volume (`/etc/istio-output-certs`)                                            | The system mounts this volume only when it detects Istio and Prometheus input is enabled. The volume provides Istio certificates to scrape application metrics that require mTLS (when the application follows a STRICT mTLS policy). |

**Key Behavior**: The `app-services` and `app-services-secure` jobs are complementary:
- `app-services`: Always present, drops `https` targets (STRICT mTLS workloads)
- `app-services-secure`: Only when Istio detected and Prometheus input enabled, keeps only `https` targets (STRICT mTLS workloads) and uses Istio certificates

This ensures workloads are scraped by exactly one job, preventing duplicate metrics.


#### OTel Log Agent

The OTel Log Agent is deployed as a DaemonSet and collects container logs using file-based collection, then forwards them to configured backends.

##### Unconditional (Always Applied)

| Resource       | Behavior                                               | Rationale                                                                                                                                                          |
|----------------|--------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Label      | `sidecar.istio.io/inject: "true"`                      | Istio sidecar injection is always enabled for the OTel Log Agent to support output mTLS communication to in-cluster backends in the Istio mesh.                  |
| Pod Annotation | `traffic.sidecar.istio.io/excludeInboundPorts: "8888"` | Excludes the metrics port from Istio sidecar interception, ensuring that monitoring systems can scrape the Log Agent's own metrics directly without mTLS overhead. |


##### Conditional (Only When Istio is Detected)

| Resource                | Behavior                                                          | Rationale                                                                                             |
|-------------------------|-------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------|
| NetworkPolicy (Ingress) | Additionally permits traffic on Istio Envoy telemetry port (15090) | When Istio is present, the sidecar's Envoy proxy exposes metrics that monitoring systems must scrape. |

#### Fluent Bit

Fluent Bit is deployed as a DaemonSet and provides legacy log collection capabilities.

##### Unconditional (Always Applied)

| Resource  | Behavior                          | Rationale                                                                                                                                 |
|-----------|-----------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Label | `sidecar.istio.io/inject: "true"` | Istio sidecar injection is always enabled for Fluent Bit to support output mTLS communication to in-cluster backends in the Istio mesh. |

##### Conditional (Only When Istio is Detected)

| Resource                | Behavior                                                          | Rationale                                                                                                                                                      |
|-------------------------|-------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------|
| NetworkPolicy (Ingress) | Additionally permits traffic on Istio Envoy telemetry port (15090) | When Istio is present, the sidecar's Envoy proxy exposes metrics that monitoring systems must scrape.                                                          |
| Pod Annotation          | `traffic.sidecar.istio.io/excludeInboundPorts: "2020, 2021"`      | Excludes the metrics port from Istio sidecar interception, ensuring that monitoring systems can scrape FluentBit's own metrics directly without mTLS overhead. |


#### Self-Monitor

The Self-Monitor is a Prometheus instance deployed as a Deployment that scrapes metrics from Telemetry components for health monitoring and alerting.

##### Unconditional (Always Applied)

| Resource  | Behavior                           | Rationale                                                                                                                                                                                                                           |
|-----------|------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Label | `sidecar.istio.io/inject: "false"` | The Self-Monitor only scrapes metrics from Telemetry components within the same namespace and does not need mTLS. Because of this, it explicitly disables Istio sidecar injection, which reduces resource overhead. |


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

Add an `istio` field to the Telemetry CR spec with separate input and output controls:

```yaml
spec:
  istio:
    input: <On | Off>   # Default: On
    output: <On | Off>  # Default: On
```

This design differentiates between input (receiving or scraping telemetry data) and output (sending telemetry data to backends), while applying globally across all telemetry components.

### Mode Semantics

**`On`**: Enable Istio integration **when Istio is present** (CRDs detected). If Istio is not installed, this mode has no effect - no Istio configurations are applied, and no labels are set.

**`Off`**: Explicitly disable Istio integration, regardless of whether Istio is present. This sets `sidecar.istio.io/inject: "false"` to ensure components do not use Istio even if it's installed.

This makes the API intuitive: `On` means "use Istio if available", `Off` means "never use Istio".

### Input Mode

Controls Istio integration for receiving or collecting telemetry data.

#### Input Mode: On (Default)

Enable Istio integration for input **when Istio is present**:

**Gateway**: 
- If Istio present: Applies DestinationRule (TLS DISABLE) for OTLP services, enables sidecar injection with traffic routing annotations
- If Istio not present: No Istio configurations, no labels set

**Metric Agent**: 
- If Istio present and at least one MetricPipeline has `input.prometheus.enabled: true`: Mounts Istio certificates, configures `app-services-secure` Prometheus scrape job, enables sidecar injection
- If Istio present but no MetricPipeline has Prometheus input enabled: Sets `sidecar.istio.io/inject: "false"` (Istio available but not needed for this component)
- If Istio not present: No Istio configurations, no labels set

**Log Agents**: 
- No effect (file-based log collection does not require Istio)

**Metric Agent Receiver Configuration**:

| Prometheus Input | Istio Present | Receiver Configuration                                                                                   |
|------------------|---------------|----------------------------------------------------------------------------------------------------------|
| Enabled          | Yes           | `app-services` (drops HTTPS targets) + `app-services-secure` (scrapes STRICT mTLS workloads with certs) |
| Enabled          | No            | `app-services` (scrapes all targets including HTTPS)                                                     |
| Disabled         | Any           | No Prometheus receivers configured                                                                       |

#### Input Mode: Off

Explicitly disable Istio integration for all input:

**Gateway**: 
- No DestinationRule, sets `sidecar.istio.io/inject: "false"`

**Metric Agent**: 
- No Istio certificates, no `app-services-secure` receiver
- Cannot scrape STRICT mTLS workloads
- `app-services` receiver scrapes all targets including HTTPS (will fail for STRICT mTLS workloads)
- Sets `sidecar.istio.io/inject: "false"`

**Log Agents**: 
- Sets `sidecar.istio.io/inject: "false"`

**Metric Agent Receiver Configuration**:

| Prometheus Input | Receiver Configuration                               |
|------------------|------------------------------------------------------|
| Enabled          | `app-services` (scrapes all targets including HTTPS) |
| Disabled         | No Prometheus receivers configured                   |

### Output Mode

Controls Istio integration for sending telemetry data to backends.

#### Output Mode: On (Default)

Enable Istio integration for output **when Istio is present**:

**All Components**: 
- If Istio present: Enables sidecar injection for all telemetry components
- If Istio not present: No Istio configurations, no labels set

**Traffic Routing Configuration** (when Istio present):
- The system analyzes each pipeline's output URL to determine which ports need traffic routing through the sidecar
- **Cluster-internal URLs** (for example, `http://otel-collector.observability.svc.cluster.local:4317`): Backend ports are added to `traffic.sidecar.istio.io/includeOutboundPorts` annotation to enable mTLS communication
- **External URLs** (for example, `https://logs.external.com`): Backend ports bypass the sidecar (direct connection, no mTLS)

#### Output Mode: Off

Explicitly disable Istio integration for all output:

**All Components**: 
- Sets `sidecar.istio.io/inject: "false"`
- Components cannot send data to STRICT mTLS backends in the Istio mesh, even if they use cluster-internal URLs

### Istio Processors

**Important**: The `istio_enrichment` and `istio_noise_filter` OTel processors are **independent** from `input` and `output` mode settings. OTel workloads include these processors in their pipelines according to the specifications below, regardless of Istio mode configuration.

**Configuration per component**:

1. **OTLP Gateway**:
   - Configure `istio_enrichment` processor **only in log pipelines** when the log pipeline is active (not present when using legacy Fluent Bit for logs).
   - Always configure `istio_noise_filter` processor in trace, metric, and log pipelines.
   - Independent of `input` and `output` settings.
   - Enriches logs with Istio service mesh metadata (workload identity, mesh attributes).
   - Filters out noisy Istio-related telemetry (health checks, internal communication).

2. **Metric Agent**:
   - Does not configure `istio_enrichment` processor.
   - Always configure `istio_noise_filter` processor in metric pipelines.
   - Independent of `input` and `output` settings.
   - Filters out noisy Istio-related metrics.

3. **OTel Log Agent**:
   - Does not configure `istio_enrichment` processor.
   - Always configure `istio_noise_filter` processor in log pipelines.
   - Independent of `input` and `output` settings.
   - Filters out noisy Istio-related logs.

4. **Fluent Bit**:
   - Fluent Bit does not support OTel processors, so neither `istio_enrichment` nor `istio_noise_filter` are applicable.

**Rationale**: The `istio_noise_filter` processor provides valuable noise reduction for all signal types. The `istio_enrichment` processor is used by the Gateway to enrich trace, metric, and log data with service mesh context. These processors operate on telemetry data within the OTel pipeline and do not require Istio certificates or sidecar injection.

### Migration Path

The proposed API provides a two-phase migration path where input remains stable while output transitions to opt-in, maintaining backward compatibility during the transition.

#### Phase 1: Introduce API with Both Defaults On

**Goal**: Provide explicit control while preserving existing behavior

**Changes**:
- Add `istio.input` and `istio.output` fields with `On | Off` modes
- Default **both** fields to `On` to ensure backward compatibility with existing behavior
- Users can explicitly opt out by setting fields to `Off`

**Benefits**:
- **Simplest possible API**: Two boolean-like fields (On/Off) control all components
- **Clear separation of concerns**: Input (receiving/scraping data) vs output (sending data to backends) matches naturally to telemetry pipeline concepts
- **Aligns with pipeline terminology**: Matches the existing `input` and `output` sections in pipeline CRDs
- **Intelligent traffic routing**: When `output: On` and Istio is present, the system analyzes pipeline URLs to configure traffic routing annotations (cluster-internal URLs route through sidecar, external URLs bypass)
- **Backward compatibility**: Default `On` for both fields ensures existing behavior is preserved
- **Flexible control**: Users can disable input and/or output independently to save resources when Istio integration is not needed

**Limitations**:
- **Global application**: All components share the same input and output settings (cannot enable Istio for only Gateway while disabling for Metric Agent)
- **URL-based heuristic**: Cluster-internal URL detection might not capture all cases (for example, ServiceEntry-backed services)

**Migration for Existing Clusters**:

For clusters with Istio:
- **No action needed**: The default `input: On, output: On` preserves current behavior
- **Optional optimization**: Users can set `input: Off` and/or `output: Off` to save resources if not needed

For clusters without Istio:
- **No action needed**: Detection logic checks for Istio CRDs before applying configurations
- **Optional explicit configuration**: Users can set `input: Off, output: Off` to document intent

#### Phase 2: Change Output Default to Off

**Goal**: Make output Istio mode opt-in by default while keeping input enabled (addresses [issue #657](https://github.com/kyma-project/telemetry-manager/issues/657))

**Changes**:
- Keep `istio.input` default as `On` (input remains enabled by default)
- Change `istio.output` default from `On` to `Off` (output becomes opt-in)
- Users who need Istio integration for output must explicitly set `output: On` in their Telemetry CR

**Rationale for Keeping Input On**:
- **Metric Agent input functionality**: The Metric Agent's ability to scrape STRICT mTLS workloads using Prometheus input is a valuable feature that users expect to work by default when Istio is present
- **Gateway input requirements**: The Gateway requires DestinationRule configuration to receive telemetry data correctly in Istio meshes
- **Minimal overhead when Istio not present**: Input mode only activates when Istio CRDs are detected, so there's no cost in non-Istio clusters
- **User expectations**: Input features (scraping mTLS workloads, receiving data in mesh) are expected to "just work" when both Istio and Telemetry Manager are installed

**Rationale for Changing Output to Off**:
- **Resource efficiency**: Output mode injects sidecars into all telemetry components, adding significant resource overhead
- **Explicit opt-in for mesh integration**: Users should consciously decide when to integrate telemetry components into the Istio mesh
- **Most common use case**: Many users send telemetry to external backends that don't require Istio mTLS

**Migration Strategy**:

1. **Announce change**: Provide advance notice (at least 2 releases) that the output default will change to `Off`
2. **Audit tooling**: Provide a command or script that checks existing clusters for:
   - Istio presence
   - Pipelines with cluster-internal output URLs
   - Generates recommended Telemetry CR configurations
3. **Documentation**: Update all examples and guides to show explicit output configuration

**Phase 2 Benefits**:
- **Resource efficiency for common case**: Most users send to external backends and won't pay sidecar overhead
- **Explicit output configuration**: Users consciously decide when to integrate with Istio mesh for output
- **Cleaner defaults**: New clusters start with minimal overhead for output while maintaining input functionality

**Phase 2 Breaking Changes**:
- Existing Telemetry CRs without explicit `istio.output` configuration will have output Istio disabled after upgrade

**Timeline**:
- **Phase 1**: Immediate implementation (`input: On, output: On` defaults)
- **Phase 2**: After Phase 1 has been stable for at least 2 releases (`input: On, output: Off` defaults)

### Backward Compatibility

#### Phase 1: Both Defaults On

The global input/output API approach ensures backward compatibility during Phase 1:

- **Default `On` for both input and output**: Preserves current behavior where Istio integration is enabled for all components when Istio CRDs are present
- **No breaking changes**: Existing Telemetry CRs without the `istio` field work with the default `On` setting, which matches current implicit behavior
- **Explicit control when needed**: Users can set `input: Off` and/or `output: Off` to disable Istio and save resources when not needed

**Comparison with Current Behavior**:

| Current (Implicit)                   | Phase 1 API Equivalent            | Behavior                                                        |
|--------------------------------------|-----------------------------------|-----------------------------------------------------------------|
| Always enabled (when Istio detected) | `input: On, output: On` (default) | Full Istio integration for all components when Istio is present |
| No explicit disable option           | `input: Off, output: Off`         | All Istio disabled, opt-out for resource savings                |

**Migration for Existing Clusters**:

For clusters with Istio:
- **No action needed**: The default `input: On, output: On` preserves current behavior
- **Optional optimization**: Users can set `input: Off` and/or `output: Off` if they want to save resources and do not need Istio integration

For clusters without Istio:
- **No action needed**: Detection logic checks for Istio CRDs before applying configurations
- **Optional explicit configuration**: Users can set `input: Off, output: Off` to document intent

#### Phase 2: Output Default Off, Input Stays On

When Phase 2 is implemented, the backward compatibility approach changes only for output:

- **Default `On` for input**: Input Istio integration remains enabled by default when Istio CRDs are present (no change from Phase 1)
- **Default `Off` for output**: Output Istio integration becomes opt-in (requires explicit `output: On`)
- **Explicit opt-in for output**: Users must explicitly set `output: On` to enable Istio sidecar injection for output traffic

**Comparison with Phase 1 Behavior**:

| Scenario                             | Phase 1 (Both On Default)         | Phase 2 (Input On, Output Off Default) |
|--------------------------------------|-----------------------------------|----------------------------------------|
| New Telemetry CR, no `istio` field   | `input: On, output: On` (default) | `input: On, output: Off` (default)     |
| Existing Telemetry CR during upgrade | No change                         | User need opt in `output: On`          |
| Explicit `istio: {output: On}`       | Behavior unchanged                | Behavior unchanged                     |
| User wants input only                | Must set `output: Off`            | No action needed (default behavior)    |
