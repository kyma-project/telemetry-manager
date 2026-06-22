---
title: Istio Mode Configuration API
status: Proposed
date: 2026-06-10
---

# Istio Mode Configuration API

## Context and Problem Statement

Telemetry Manager currently auto-detects Istio by checking for Istio CRDs (`*.istio.io`) in the cluster and automatically applies Istio-specific resources when detected. This includes sidecar injection labels, PeerAuthentication resources, DestinationRules, traffic routing annotations, and certificate volume mounts across telemetry components (OTLP Gateway, Metric Agent, OTel Log Agent, Fluent Bit, and Self-Monitor).

Although automatic detection provides convenience, several operational challenges exist:

- No explicit control: Because users cannot disable Istio mode even when not needed, unnecessary resource overhead results from sidecar containers, certificate management, and Istio-specific network policies.

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

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Label | `sidecar.istio.io/inject: "true"` | Istio sidecar injection is always enabled for the OTLP Gateway to support outbound mTLS communication to in-cluster backends in the Istio mesh. |

##### Conditional (Only When Istio is Detected)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Annotation | `sidecar.istio.io/interceptionMode: TPROXY` | Configures the Istio sidecar to use transparent proxy mode for traffic interception. |
| Pod Annotation | `traffic.sidecar.istio.io/includeInboundPorts: ""` | Excludes all inbound ports from Istio sidecar interception, ensuring direct access to OTLP ingestion endpoints without mTLS overhead. |
| PeerAuthentication | PERMISSIVE mTLS mode | This configuration supports both plain-text and mTLS connections to the OTLP Gateway. Applications can send data over plain-text because of node-local ingestion and can use mTLS communication. |
| DestinationRule | `TLS mode: DISABLE` for all OTLP Services | Because other components must connect without requiring mTLS, the OTLP Gateway receives telemetry data over plain-text on the ingestion path because of node-local routing. Disabling TLS for client connections to these services supports this requirement. |
| NetworkPolicy (Ingress) | Additionally allows traffic on Istio Envoy telemetry port (15090) | When Istio is present, the sidecar's Envoy proxy exposes metrics that monitoring systems must scrape. |

#### Metric Agent

The Metric Agent is deployed as a DaemonSet and scrapes Prometheus metrics from workloads in the cluster, then forwards them to configured backends.

##### Unconditional (Always Applied)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Label | `sidecar.istio.io/inject: "true"` | Istio sidecar injection is always enabled for the Metric Agent to support outbound mTLS communication to in-cluster backends in the Istio mesh. |

##### Conditional (Only When Istio is Detected)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Annotation | `traffic.sidecar.istio.io/includeOutboundIPRanges: ""` | Bypasses Istio sidecar interception for most outbound traffic. Prometheus scraping of Istio control plane and Envoy metrics requires direct access to metric endpoints. These endpoints are not reachable through the sidecar proxy. |
| Pod Annotation | `traffic.sidecar.istio.io/includeOutboundPorts: "{backend_ports}"` | Ensures that traffic to configured backends (such as OTLP Gateway, in-cluster Prometheus, or other OTel Collectors) goes through the Istio sidecar for mTLS. The reconciliation loop populates this with the actual backend ports from MetricPipeline configurations. |
| Pod Annotation | `traffic.sidecar.istio.io/excludeInboundPorts: "8888"` | Excludes the metrics port from Istio sidecar interception, ensuring that monitoring systems can scrape the Metric Agent's own metrics directly without mTLS overhead. |
| Pod Annotation | `proxy.istio.io/config` | Configures the Istio sidecar to write TLS certificates to the shared volume at `/etc/istio-output-certs`, which the Metric Agent uses for mTLS scraping of application metrics. |
| Pod Annotation | `sidecar.istio.io/userVolumeMount` | Mounts the Istio certificate volume into the Istio sidecar container. |
| Prometheus Scrape Config | `app-service-secure` job | Configures scraping for application services with STRICT mTLS policies using Istio certificates. The Metric Agent can scrape workloads in the Istio mesh that require mTLS authentication. |
| NetworkPolicy (Ingress) | Additionally allows traffic on Istio Envoy telemetry port (15090) | When Istio is present, the sidecar's Envoy proxy exposes metrics that monitoring systems must scrape. |
| Volume Mount | Istio certificates volume (`/etc/istio-output-certs`) | The Metric Agent always mounts Istio certificates to scrape application metrics that require mTLS (when the application follows a STRICT mTLS policy). |


#### OTel Log Agent

The OTel Log Agent is deployed as a DaemonSet and collects container logs using file-based collection, then forwards them to configured backends.

##### Unconditional (Always Applied)

| Resource | Behavior | Rationale                                                                                                                                           |
|----------|----------|-----------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Label | `sidecar.istio.io/inject: "true"` | Istio sidecar injection is always enabled for the OTel Log Agent to support outbound mTLS communication to in-cluster backends in the Istio mesh.      |
| Pod Annotation | `traffic.sidecar.istio.io/excludeInboundPorts: "8888"` | Excludes the metrics port from Istio sidecar interception, ensuring that monitoring systems can scrape the Log Agent's own metrics directly without mTLS overhead. |


##### Conditional (Only When Istio is Detected)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| NetworkPolicy (Ingress) | Additionally allows traffic on Istio Envoy telemetry port (15090) | When Istio is present, the sidecar's Envoy proxy exposes metrics that monitoring systems must scrape. |

#### Fluent Bit

Fluent Bit is deployed as a DaemonSet and provides legacy log collection capabilities.

##### Unconditional (Always Applied)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Label | `sidecar.istio.io/inject: "true"` | Istio sidecar injection is always enabled for Fluent Bit to support outbound mTLS communication to in-cluster backends in the Istio mesh. |

##### Conditional (Only When Istio is Detected)

| Resource | Behavior                                                          | Rationale                                                                                                                                           |
|----------|-------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------|
| NetworkPolicy (Ingress) | Additionally allows traffic on Istio Envoy telemetry port (15090) | When Istio is present, the sidecar's Envoy proxy exposes metrics that monitoring systems must scrape.                                     |
| Pod Annotation | `traffic.sidecar.istio.io/excludeInboundPorts: "2020, 2021"`      | Excludes the metrics port from Istio sidecar interception, ensuring that monitoring systems can scrape FluentBit's own metrics directly without mTLS overhead. |


#### Self-Monitor

The Self-Monitor is a Prometheus instance deployed as a Deployment that scrapes metrics from Telemetry components for health monitoring and alerting.

##### Unconditional (Always Applied)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Label | `sidecar.istio.io/inject: "false"` | The Self-Monitor only scrapes metrics from Telemetry components within the same namespace and does not need mTLS. Therefore, it explicitly disables Istio sidecar injection. Running without the sidecar reduces resource overhead. |


##### Conditional (Only When Istio is Detected)

No Istio-specific resources are created for the Self-Monitor because it explicitly disables sidecar injection.

## Summary Table

| Component      | Sidecar Injection | Istio Certificates | Special Annotations           | Istio-Specific Resources                                       |
|----------------|-------------------|--------------------|-------------------------------|----------------------------------------------------------------|
| OTLP Gateway   | Always enabled    | Not used           | None                          | PeerAuthentication (PERMISSIVE), DestinationRule (TLS DISABLE) |
| Metric Agent   | Always enabled    | Always mounted     | Conditional (traffic routing) | None                                                           |
| OTel Log Agent | Always enabled    | Not used           | None                          | None                                                           |
| Fluent Bit     | Always enabled    | Not used           | None                          | None                                                           |
| Self-Monitor   | Always disabled   | Not used           | None                          | None                                                           |


## Proposed API

To provide explicit control over Istio integration, we propose adding an `istio` field to the Telemetry CR spec with an enum-based mode configuration. This field allows users to control Istio mode globally for all telemetry components.

### API Schema

```yaml
spec:
  istio:
    mode: <On | Auto | Off>  # Default: On
```

- **On** (Default): Force Istio integration on all telemetry components when Istio is present.
  - The system checks for Istio CRDs (`*.istio.io`) in the cluster.
  - If Istio is present, the system applies Istio sidecar injection, certificates, annotations, and resources to all components.
  - If Istio is not present, the system does not apply Istio configurations (behaves like Off mode).
  - All components receive full Istio integration regardless of pipeline configurations.
  - Metric Agent includes the `app-service-secure` Prometheus scrape job for STRICT mTLS workloads.
  - This is the default mode to ensure backward compatibility with existing clusters.
- **Auto**: Automatically detect Istio integration requirements on a per-component basis.
  - The system checks for Istio CRDs (`*.istio.io`) in the cluster.
  - The system analyzes pipeline configurations to determine which components require Istio.
  - The system applies Istio-specific resources only to components that require them based on their configuration.
  - The system enables the Metric Agent if `input.prometheus.enabled: true` or output to cluster-internal backend. The input `input.istio.enabled` has no effect on Istio auto-detection.
  - The system enables Log Agents if output to cluster-internal backend.
  - The system enables OTLP Gateway if output to cluster-internal backend.
- **Off**: Disable Istio integration completely across all components.
  - The system explicitly sets the sidecar injection label to `"false"` for all components.
  - The system does not apply Istio certificates, annotations, or resources.
  - The system removes the `app-service-secure` Prometheus scrape job from the Metric Agent when `prometheus` input is enabled.
  - Components cannot communicate with STRICT mTLS workloads in the Istio mesh.

## User Examples

### Example 1: Force Enable on All Components (Default Behavior)

Enable Istio mode on all telemetry components (this is the default):

```yaml
apiVersion: operator.kyma-project.io/v1beta1
kind: Telemetry
metadata:
  name: default
  namespace: kyma-system
spec:
  istio:
    mode: On  # This is the default
  metric:
    collectionInterval: 30s
```

Or simply omit the field (defaults to On):

```yaml
apiVersion: operator.kyma-project.io/v1beta1
kind: Telemetry
metadata:
  name: default
  namespace: kyma-system
spec:
  metric:
    collectionInterval: 30s
```

**Behavior**: When `mode: On` (or omitted), the system checks for Istio presence and applies configurations accordingly:

1. **Istio CRD Detection**: Checks if Istio CRDs (`*.istio.io`) are present in the cluster
2. **If Istio is present**:
   - All components receive Istio sidecar injection (`sidecar.istio.io/inject: "true"`)
   - All applicable Istio resources created (PeerAuthentication, DestinationRule)
   - All Istio traffic routing annotations applied
   - Metric Agent mounts Istio certificate volumes and configures `app-service-secure` scrape job
   - This ensures full Istio integration for all components regardless of pipeline configurations
3. **If Istio is not present**:
   - No Istio configurations are applied to any component
   - Components run without Istio sidecars, annotations, or resources
   - Behaves like `Off` mode

This default mode is safe for both Istio-enabled and non-Istio clusters, providing backward compatibility.

**Example Pipeline Configurations**:

All pipelines work with Istio enabled (when Istio is present):

```yaml
apiVersion: telemetry.kyma-project.io/v1beta1
kind: LogPipeline
metadata:
  name: external-logs
spec:
  output:
    http:
      host:
        value: logs.external.com
      port: "443"
# Result: Log Agents have Istio enabled (can communicate with both external and cluster-internal backends)
```

```yaml
apiVersion: telemetry.kyma-project.io/v1beta1
kind: MetricPipeline
metadata:
  name: cluster-metrics
spec:
  input:
    prometheus:
      enabled: true
  output:
    otlp:
      endpoint:
        value: http://otel-collector.observability.svc.cluster.local:4317
# Result: Metric Agent has Istio enabled with app-service-secure job (can scrape STRICT mTLS workloads and communicate with cluster backends)
```

### Example 2: Per-Component Auto-Detection

Automatically detect Istio requirements per component:

```yaml
apiVersion: operator.kyma-project.io/v1beta1
kind: Telemetry
metadata:
  name: default
  namespace: kyma-system
spec:
  istio:
    mode: Auto
  metric:
    collectionInterval: 30s
```

**Behavior**: When `mode: Auto`, the system intelligently detects whether Istio integration is needed per component:

1. **Istio CRD Detection**: Checks if Istio CRDs (`*.istio.io`) are present in the cluster
2. **Pipeline Analysis**: Analyzes pipeline configurations to determine if they require Istio:
   - **Metric Agent**: Requires Istio if MetricPipelines have `input.prometheus.enabled: true` (requires Istio for `app-service-secure` scrape job to access STRICT mTLS workloads) or output to cluster-internal backends
   - **Log Agents**: Require Istio if LogPipelines output to cluster-internal backends
   - **OTLP Gateway**: Requires Istio if TracePipelines output to cluster-internal backends
   - Note: `input.istio.enabled` in MetricPipeline has no effect on Istio auto-detection
3. **Smart Activation**: Applies Istio-specific resources (sidecar injection, PeerAuthentication, DestinationRule, traffic annotations, certificate volumes) only when both conditions are met:
   - Istio is present in the cluster
   - At least one pipeline requires Istio integration

**Example Pipeline Configurations**:

LogPipeline with cluster-internal output (triggers Istio for Log Agents):
```yaml
apiVersion: telemetry.kyma-project.io/v1beta1
kind: LogPipeline
metadata:
  name: backend
spec:
  output:
    http:
      host:
        value: fluentd.logging.svc.cluster.local
      port: "8080"
```

MetricPipeline with Prometheus input (triggers Istio for Metric Agent):
```yaml
apiVersion: telemetry.kyma-project.io/v1beta1
kind: MetricPipeline
metadata:
  name: app-metrics
spec:
  input:
    prometheus:
      enabled: true
    # Note: input.istio.enabled has no effect on Istio auto-detection
  output:
    otlp:
      endpoint:
        value: https://external-metrics.com
```

MetricPipeline with cluster-internal output (triggers Istio for Metric Agent):
```yaml
apiVersion: telemetry.kyma-project.io/v1beta1
kind: MetricPipeline
metadata:
  name: cluster-backend
spec:
  output:
    otlp:
      endpoint:
        value: http://otel-collector.observability.svc.cluster.local:4317
```

TracePipeline with cluster-internal output (triggers Istio for OTLP Gateway):
```yaml
apiVersion: telemetry.kyma-project.io/v1beta1
kind: TracePipeline
metadata:
  name: backend
spec:
  output:
    otlp:
      endpoint:
        value: http://jaeger-collector.observability.svc.cluster.local:4317
```

### Example 3: Force Disable on All Components

Force Istio mode off on all components:

```yaml
apiVersion: operator.kyma-project.io/v1beta1
kind: Telemetry
metadata:
  name: default
  namespace: kyma-system
spec:
  istio:
    mode: Off
  metric:
    collectionInterval: 30s
```

**Behavior**: 
- The system sets the label `sidecar.istio.io/inject` to `"false"`.
- The system does not create PeerAuthentication or DestinationRule resources.
- The system does not apply Istio traffic routing annotations.
- The system does not mount Istio certificate volumes.
- The system does not configure the `app-service-secure` Prometheus receiver for the metric agent.
- The system can still scrape Istio metrics if Istio is present in the cluster.

**Example Pipeline Configurations**:

Even with cluster-internal outputs, Istio is disabled:

LogPipeline (Istio disabled despite cluster-internal output):
```yaml
apiVersion: telemetry.kyma-project.io/v1beta1
kind: LogPipeline
metadata:
  name: backend
spec:
  output:
    http:
      host:
        value: fluentd.logging.svc.cluster.local
      port: "8080"
# Result: Log agents have sidecar.istio.io/inject: "false"
```

MetricPipeline with Prometheus input (Istio disabled, no app-service-secure job):
```yaml
apiVersion: telemetry.kyma-project.io/v1beta1
kind: MetricPipeline
metadata:
  name: app-metrics
spec:
  input:
    prometheus:
      enabled: true
  output:
    otlp:
      endpoint:
        value: https://external-metrics.com
# Result: Metric Agent has sidecar.istio.io/inject: "false"
# app-service-secure scrape job is removed - cannot scrape STRICT mTLS workloads
```

### Implementation Impact

When `istio.mode` is set, the reconciliation logic changes as follows:

#### Detection Logic for Auto Mode

When `mode: Auto`, the system performs detection for Istio required:

1. **Istio CRD Check**: Scan for Istio CRDs (`*.istio.io`) in the cluster
2. **Pipeline Analysis**: Check if any active pipelines require Istio:
   - **Metric Agent**: MetricPipelines with `input.prometheus.enabled: true` (requires `app-service-secure` scrape job for STRICT mTLS workloads) or MetricPipelines with cluster-internal output URLs
   - **Log Agents**: LogPipelines with cluster-internal output URLs
   - **OTLP Gateway**: TracePipelines with cluster-internal output URLs
   - Note: `input.istio.enabled` in MetricPipeline does not affect Istio auto-detection in Auto mode
3. **Decision**: Enable Istio integration only if BOTH conditions are true:
   - Istio CRDs are present
   - At least one pipeline requires Istio integration

#### Components Affected

All reconcilers that create or configure telemetry components must respect the `istio.mode` setting:

1. **OTLP Gateway Reconciler**
   - When `mode: On`: Apply all Istio resources if Istio CRDs are present (PeerAuthentication, DestinationRule, sidecar injection, annotations). If Istio CRDs are not present, skip Istio configurations.
   - When `mode: Auto`: Apply Istio resources only if detection logic confirms Istio is needed (Istio CRDs present and TracePipelines with cluster-internal output URLs)
   - When `mode: Off`: Skip PeerAuthentication creation, skip DestinationRule creation, set sidecar injection label to `"false"`

2. **Metric Agent Reconciler**
   - When `mode: On`: Apply all Istio configurations including `app-service-secure` scrape job if Istio CRDs are present. If Istio CRDs are not present, skip Istio configurations.
   - When `mode: Auto`: Apply Istio configurations only if detection logic confirms Istio is needed (Istio CRDs present and (Prometheus input enabled or cluster-internal output URL configured)).
   - When `mode: Off`: Skip Istio traffic routing annotations, skip certificate volume mounts, remove `app-service-secure` scrape job, set sidecar injection label to `"false"`

3. **OTel Log Agent Reconciler**
   - When `mode: On`: Apply sidecar injection if Istio CRDs are present. If Istio CRDs are not present, skip sidecar injection.
   - When `mode: Auto`: Apply sidecar injection only if detection logic confirms Istio is needed (Istio CRDs present and LogPipelines with cluster-internal output URLs)
   - When `mode: Off`: Set sidecar injection label to `"false"`

4. **Fluent Bit Reconciler**
   - When `mode: On`: Apply sidecar injection if Istio CRDs are present. If Istio CRDs are not present, skip sidecar injection.
   - When `mode: Auto`: Apply sidecar injection only if detection logic confirms Istio is needed (Istio CRDs present and LogPipelines with cluster-internal output URLs)
   - When `mode: Off`: Set sidecar injection label to `"false"`

5. **NetworkPolicy Reconcilers**
   - When `mode: On`: Include Istio Envoy port (15090) for all components if Istio CRDs are present. If Istio CRDs are not present, omit the port.
   - When `mode: Auto`: Include Istio Envoy port (15090) in ingress rules only for components where detection logic confirms Istio is needed
   - When `mode: Off`: Omit Istio Envoy port (15090) from all ingress rules


### Migration Path

The proposed API provides a smooth migration path:

#### Introduce Three-Mode Enum with On Default (Current Proposal)
- Add `istio.mode` field with `On` as the default value
- Three modes provide flexibility:
  - `On` (default): Enable Istio on all components when Istio is present in the cluster
  - `Auto`: Per-component detection based on pipeline requirements
  - `Off`: Force disable Istio on all components
- Default `On` ensures existing Istio-enabled clusters continue to work without changes
- Default `On` is safe for non-Istio clusters (automatically detects absence and skips configurations)
- Users can opt into `Auto` mode for optimized resource usage when they understand their pipeline destinations
- Clear semantics: `On` = enable all if Istio present, `Auto` = smart per-component detect, `Off` = force none

**Pros:**
- Zero disruption during initial rollout: Default `On` ensures all Istio-enabled clusters continue working
- Safe for non-Istio clusters: `On` mode checks for Istio presence, preventing unnecessary configurations
- Maximum compatibility: Default mode ensures Istio integration when Istio is installed
- Opt-in optimization: Users can switch to `Auto` mode to reduce resource overhead when they know their pipelines
- Per-component resource management in Auto mode: Istio integration enabled only on components with specific requirements
- Explicit control: `Off` mode provides override for users who want to disable Istio completely
- Extensible: Enum design allows future modes without breaking changes
- Low risk: Enum provides type safety and clear intent

**Cons:**
- Default `On` may apply Istio to components that don't need it in Istio-enabled clusters (higher resource usage)
- Auto mode requires per-component detection logic (more complex implementation)
- Per-component state in Auto mode: Components may have different Istio states simultaneously
- Users must explicitly opt into Auto mode to benefit from resource optimization

### Backward Compatibility

The three-mode enum approach with `On` default ensures full backward compatibility:

- Existing Telemetry CRs without the `istio` field continue to have Istio enabled on all components when Istio is present (default `mode: On`)
- This matches the current behavior where Istio is auto-detected (via CRD presence) and applied to all components
- The `On` mode is safe for non-Istio clusters: it checks for Istio CRDs and skips Istio configurations when Istio is not installed
- Users who upgrade to the new API version can explicitly control Istio mode without modifying existing CRs
- The default `On` behavior ensures maximum compatibility with existing Istio-enabled clusters while being safe for non-Istio clusters
- The enum provides three clear modes that cover all operational scenarios:
  - `On`: Enable for all components when Istio is present (default, backward compatible with current behavior)
  - `Auto`: Opt-in per-component detection based on pipeline requirements (new optimization capability)
  - `Off`: Explicit force-disable for all components regardless of Istio presence (opt-out)
- No breaking changes: existing behavior is preserved with the default `On` setting
- Users can opt into `Auto` mode when they want optimized resource usage and understand their pipeline destinations
