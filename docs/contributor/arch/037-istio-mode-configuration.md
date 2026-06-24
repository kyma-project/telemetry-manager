---
title: Istio Mode Configuration API
status: Proposed
date: 2026-06-10
---

# Istio Mode Configuration API

## Context and Problem Statement

Telemetry Manager currently auto-detects Istio by checking for Istio CRDs (`*.istio.io`) in the cluster and automatically applies Istio-specific resources when detected. This includes sidecar injection labels, DestinationRules, traffic routing annotations, and certificate volume mounts across telemetry components (OTLP Gateway, Metric Agent, OTel Log Agent, Fluent Bit, and Self-Monitor).

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

| Resource  | Behavior                          | Rationale                                                                                                                                       |
|-----------|-----------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Label | `sidecar.istio.io/inject: "true"` | Istio sidecar injection is always enabled for the OTLP Gateway to support outbound mTLS communication to in-cluster backends in the Istio mesh. |

##### Conditional (Only When Istio is Detected)

| Resource                | Behavior                                                          | Rationale                                                                                                                                                                                                                                                     |
|-------------------------|-------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Annotation          | `sidecar.istio.io/interceptionMode: TPROXY`                       | Configures the Istio sidecar to use transparent proxy mode for traffic interception.                                                                                                                                                                          |
| Pod Annotation          | `traffic.sidecar.istio.io/includeInboundPorts: ""`                | Excludes all inbound ports from Istio sidecar interception, ensuring direct access to OTLP ingestion endpoints without mTLS overhead.                                                                                                                         |
| PeerAuthentication      | PERMISSIVE mTLS mode                                              | This configuration supports both plain-text and mTLS connections to the OTLP Gateway. Applications can send data over plain-text because of node-local ingestion and can use mTLS communication.                                                              |
| DestinationRule         | `TLS mode: DISABLE` for all OTLP Services                         | Because other components must connect without requiring mTLS, the OTLP Gateway receives telemetry data over plain-text on the ingestion path because of node-local routing. Disabling TLS for client connections to these services supports this requirement. |
| NetworkPolicy (Ingress) | Additionally allows traffic on Istio Envoy telemetry port (15090) | When Istio is present, the sidecar's Envoy proxy exposes metrics that monitoring systems must scrape.                                                                                                                                                         |

#### Metric Agent

The Metric Agent is deployed as a DaemonSet and scrapes Prometheus metrics from workloads in the cluster, then forwards them to configured backends.

##### Unconditional (Always Applied)

| Resource  | Behavior                          | Rationale                                                                                                                                       |
|-----------|-----------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Label | `sidecar.istio.io/inject: "true"` | Istio sidecar injection is always enabled for the Metric Agent to support outbound mTLS communication to in-cluster backends in the Istio mesh. |

##### Conditional (Only When Istio is Detected)

| Resource                 | Behavior                                                           | Rationale                                                                                                                                                                                                                                                             |
|--------------------------|--------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Annotation           | `traffic.sidecar.istio.io/includeOutboundIPRanges: ""`             | Bypasses Istio sidecar interception for most outbound traffic. Prometheus scraping of Istio control plane and Envoy metrics requires direct access to metric endpoints. These endpoints are not reachable through the sidecar proxy.                                  |
| Pod Annotation           | `traffic.sidecar.istio.io/includeOutboundPorts: "{backend_ports}"` | Ensures that traffic to configured backends (such as OTLP Gateway, in-cluster Prometheus, or other OTel Collectors) goes through the Istio sidecar for mTLS. The reconciliation loop populates this with the actual backend ports from MetricPipeline configurations. |
| Pod Annotation           | `traffic.sidecar.istio.io/excludeInboundPorts: "8888"`             | Excludes the metrics port from Istio sidecar interception, ensuring that monitoring systems can scrape the Metric Agent's own metrics directly without mTLS overhead.                                                                                                 |
| Pod Annotation           | `proxy.istio.io/config`                                            | Configures the Istio sidecar to write TLS certificates to the shared volume at `/etc/istio-output-certs`, which the Metric Agent uses for mTLS scraping of application metrics.                                                                                       |
| Pod Annotation           | `sidecar.istio.io/userVolumeMount`                                 | Mounts the Istio certificate volume into the Istio sidecar container.                                                                                                                                                                                                 |
| Prometheus Scrape Config | `app-services-secure` job                                           | Configures scraping for application services with STRICT mTLS policies using Istio certificates. The Metric Agent can scrape workloads in the Istio mesh that require mTLS authentication.                                                                            |
| NetworkPolicy (Ingress)  | Additionally allows traffic on Istio Envoy telemetry port (15090)  | When Istio is present, the sidecar's Envoy proxy exposes metrics that monitoring systems must scrape.                                                                                                                                                                 |
| Volume Mount             | Istio certificates volume (`/etc/istio-output-certs`)              | The Metric Agent always mounts Istio certificates to scrape application metrics that require mTLS (when the application follows a STRICT mTLS policy).                                                                                                                |


#### OTel Log Agent

The OTel Log Agent is deployed as a DaemonSet and collects container logs using file-based collection, then forwards them to configured backends.

##### Unconditional (Always Applied)

| Resource       | Behavior                                               | Rationale                                                                                                                                                          |
|----------------|--------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Label      | `sidecar.istio.io/inject: "true"`                      | Istio sidecar injection is always enabled for the OTel Log Agent to support outbound mTLS communication to in-cluster backends in the Istio mesh.                  |
| Pod Annotation | `traffic.sidecar.istio.io/excludeInboundPorts: "8888"` | Excludes the metrics port from Istio sidecar interception, ensuring that monitoring systems can scrape the Log Agent's own metrics directly without mTLS overhead. |


##### Conditional (Only When Istio is Detected)

| Resource                | Behavior                                                          | Rationale                                                                                             |
|-------------------------|-------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------|
| NetworkPolicy (Ingress) | Additionally allows traffic on Istio Envoy telemetry port (15090) | When Istio is present, the sidecar's Envoy proxy exposes metrics that monitoring systems must scrape. |

#### Fluent Bit

Fluent Bit is deployed as a DaemonSet and provides legacy log collection capabilities.

##### Unconditional (Always Applied)

| Resource  | Behavior                          | Rationale                                                                                                                                 |
|-----------|-----------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Label | `sidecar.istio.io/inject: "true"` | Istio sidecar injection is always enabled for Fluent Bit to support outbound mTLS communication to in-cluster backends in the Istio mesh. |

##### Conditional (Only When Istio is Detected)

| Resource                | Behavior                                                          | Rationale                                                                                                                                                      |
|-------------------------|-------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------|
| NetworkPolicy (Ingress) | Additionally allows traffic on Istio Envoy telemetry port (15090) | When Istio is present, the sidecar's Envoy proxy exposes metrics that monitoring systems must scrape.                                                          |
| Pod Annotation          | `traffic.sidecar.istio.io/excludeInboundPorts: "2020, 2021"`      | Excludes the metrics port from Istio sidecar interception, ensuring that monitoring systems can scrape FluentBit's own metrics directly without mTLS overhead. |


#### Self-Monitor

The Self-Monitor is a Prometheus instance deployed as a Deployment that scrapes metrics from Telemetry components for health monitoring and alerting.

##### Unconditional (Always Applied)

| Resource  | Behavior                           | Rationale                                                                                                                                                                                                                           |
|-----------|------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Pod Label | `sidecar.istio.io/inject: "false"` | The Self-Monitor only scrapes metrics from Telemetry components within the same namespace and does not need mTLS. Therefore, it explicitly disables Istio sidecar injection. Running without the sidecar reduces resource overhead. |


##### Conditional (Only When Istio is Detected)

No Istio-specific resources are created for the Self-Monitor because it explicitly disables sidecar injection.

## Summary Table

| Component      | Sidecar Injection | Istio Certificates | Special Annotations           | Istio-Specific Resources       |
|----------------|-------------------|--------------------|-------------------------------|--------------------------------|
| OTLP Gateway   | Always enabled    | Not used           | None                          | DestinationRule (TLS DISABLE)  |
| Metric Agent   | Always enabled    | Always mounted     | Conditional (traffic routing) | None                           |
| OTel Log Agent | Always enabled    | Not used           | None                          | None                           |
| Fluent Bit     | Always enabled    | Not used           | None                          | None                           |
| Self-Monitor   | Always disabled   | Not used           | None                          | None                           |


## Proposed API

To provide explicit control over Istio integration, we propose adding an `istio` field to the Telemetry CR spec with separate ingestion and export controls. This design differentiates between ingestion (receiving or scraping telemetry data) and export (sending telemetry data to backends), while applying globally across all telemetry components.

### API Schema

```yaml
spec:
  istio:
    ingestion: <On | Off>  # Default: On
    export: <On | Off>     # Default: On
```

### Mode Definitions

The `ingestion` and `export` fields control Istio integration independently across all components:

#### Ingestion Mode

Controls Istio integration for receiving or collecting telemetry data:

- **On** (Default): Enable Istio integration for ingestion paths for components that require it, **only when Istio CRDs are present in the cluster**.
  - **Gateway**: If Istio CRDs detected, applies DestinationRule (TLS DISABLE) for OTLP services. Enables sidecar injection with traffic routing annotations (exclude inbound ports). If Istio CRDs not present, skips all Istio configurations.
  - **Metric Agent**: If Istio CRDs detected AND **at least one MetricPipeline has `input.prometheus.enabled: true`**, mounts Istio certificates, configures `app-services-secure` Prometheus scrape job, applies traffic routing annotations for certificate access. Can scrape STRICT mTLS workloads. If Istio CRDs not present OR no MetricPipeline has Prometheus input enabled, skips Istio ingestion configurations.
  - **Log Agents**: No effect (collect logs from node files, not from workloads requiring mTLS).
  
- **Off**: Explicitly disable Istio integration for all ingestion paths, regardless of Istio CRD presence.
  - **Gateway**: No DestinationRule, no sidecar injection.
  - **Metric Agent**: 
    - No Istio certificates, no `app-services-secure` receiver. Cannot scrape STRICT mTLS workloads.
    - The `app-services` receiver is configured to scrape application services without mTLS only.
  - **Log Agents**: No effect.

### Prometheus Input Detection for Metric Agent Ingestion

When `ingestion: On`, the Metric Agent checks if Istio is needed for ingestion:

1. **Istio CRD Detection**: Check if Istio CRDs (`*.istio.io`) are present in the cluster.
2. **Pipeline Analysis**: Scan all active MetricPipelines for `input.prometheus.enabled: true`.
3. **Decision**: If **Istio CRDs are present** AND **at least one** MetricPipeline has `input.prometheus.enabled: true`, enable Istio for ingestion (mount certificates, configure `app-services-secure` job).
4. **If Istio CRDs not present OR no Prometheus input**: Skip Istio ingestion configurations for the Metric Agent.

This ensures Istio certificates and the `app-services-secure` job are only configured when Istio is actually installed AND needed for scraping STRICT mTLS workloads.

### Metric Agent Receiver Configuration Based on Ingestion Mode

The Metric Agent's Prometheus receiver configuration depends on the `ingestion` mode and Prometheus input enablement:

| Ingestion Mode | Prometheus Input | Receiver Configuration                                                                                      |
|----------------|------------------|-------------------------------------------------------------------------------------------------------------|
| **On**         | Enabled          | `app-services` (scrapes application services without mTLS) + `app-services-secure` (scrapes STRICT mTLS workloads with Istio certificates) |
| **On**         | Disabled         | No Prometheus receivers configured                                                                          |
| **Off**        | Enabled          | `app-services` (scrapes application services without mTLS)                                                  |
| **Off**        | Disabled         | No Prometheus receivers configured                                                                          |

**Key difference**: When `ingestion: Off` but Prometheus input is enabled:
- The `app-services-secure` receiver is **removed** (no STRICT mTLS scraping capability).
- The `app-services` receiver scrapes only application services without mTLS.
- Cannot scrape workloads with STRICT mTLS policies.

### Istio Processors (Independent of Istio Mode)

**Important**: The `istio_enrichment` and `istio_noise_filter` processors for all OTel workloads (Gateway, Metric Agent, Log Agents) remain **independent** from the `ingestion` and `export` settings. These processors operate on telemetry data within the OTel pipeline and must **always be configured** regardless of Istio mode settings.

#### istio_enrichment Processor

**Purpose**: Enriches telemetry data with Istio-related attributes (for example, service mesh metadata, workload identity).

**Rationale**: 
- Provides valuable context about the service mesh environment, even when STRICT mTLS scraping or sidecar injection is not required.
- Operates on telemetry data flowing through the OTel pipeline and does not require Istio certificates or sidecar injection.
- Enrichment data is useful for correlating telemetry across service mesh components, regardless of whether the component itself uses Istio for communication.

**Configuration**:
- **Gateway**: Always includes `istio_enrichment` processor in trace, metric, and log pipelines.
- **Metric Agent**: Always includes `istio_enrichment` processor in metric pipelines.
- **Log Agents**: Always include `istio_enrichment` processor in log pipelines.

#### istio_noise_filter Processor

**Purpose**: Filters out noisy or redundant Istio-related telemetry data (for example, health check traces, internal Istio communication).

**Rationale**: 
- Reduces telemetry volume by filtering out known noise patterns from Istio infrastructure.
- Operates on telemetry data within the OTel pipeline and does not require Istio certificates or sidecar injection.
- Noise filtering is valuable regardless of whether Istio is used for communication (sidecar injection, mTLS).

**Configuration**:
- **Gateway**: Always includes `istio_noise_filter` processor in trace, metric, and log pipelines.
- **Metric Agent**: Always includes `istio_noise_filter` processor in metric pipelines.
- **Log Agents**: Always include `istio_noise_filter` processor in log pipelines.

This ensures consistent telemetry enrichment and noise filtering across all components, independent of Istio integration mode.

#### Export Mode

Controls Istio integration for sending telemetry data to backends:

- **On** (Default): Enable Istio integration for export paths when the pipeline has a cluster-internal output endpoint, **only when Istio CRDs are present in the cluster**.
  - **All Components**: The system checks each pipeline's output URL:
    - **Cluster-internal URL detected** (for example, `http://otel-collector.observability.svc.cluster.local:4317`): If Istio CRDs present, enables sidecar injection and configures traffic routing annotations to route backend traffic through Istio sidecar for mTLS communication. If Istio CRDs not present, skips Istio configurations.
    - **External URL detected** (for example, `https://logs.external.com`): Skips Istio sidecar injection and traffic routing for that component (no mTLS needed for external backends).
  - **Per-component decision**: Each component (Gateway, Metric Agent, Log Agents) independently checks its pipelines' output URLs and enables Istio only if Istio CRDs are present AND at least one pipeline uses a cluster-internal backend.
  
- **Off**: Explicitly disable Istio integration for all export paths, regardless of Istio CRD presence.
  - **All Components**: Sets sidecar injection label to `"false"`. Components cannot send data to STRICT mTLS backends in the Istio mesh, even if they use cluster-internal URLs.

### Cluster-Internal URL Detection

The system detects cluster-internal URLs using the following patterns:

- **Kubernetes service DNS names**: 
  - `<service>.<namespace>.svc.cluster.local`
  - `<service>.<namespace>.svc`
  - `<service>.<namespace>`
  - `<service>` (same namespace)
- **Kubernetes ClusterIP addresses**: IP addresses in the cluster's service CIDR range

When `export: On`, the system scans all active pipelines for each component:

- **Gateway**: Scans all pipeline `output.otlp.endpoint` values
- **Metric Agent**: Scans all MetricPipeline `output.otlp.endpoint` values
- **Log Agents**: Scans all LogPipeline `output.http.host` values

If **at least one** pipeline has a cluster-internal URL, the component enables Istio for export.

#### Edge Cases and Limitations

**Istio ServiceEntry Resources**: The cluster-internal URL detection cannot identify external services that are registered as internal through Istio ServiceEntry resources. A ServiceEntry adds entries to Istio's internal service registry, making external services appear as if they are part of the mesh.

**Example scenario**:
```yaml
apiVersion: networking.istio.io/v1beta1
kind: ServiceEntry
metadata:
  name: external-backend
spec:
  hosts:
  - external-backend.example.com  # External domain
  location: MESH_EXTERNAL
  ports:
  - number: 443
    name: https
    protocol: HTTPS
  resolution: DNS
```

When a pipeline references `https://external-backend.example.com:443`, the URL detection logic identifies it as an external URL (does not match Kubernetes DNS patterns). However, if a ServiceEntry exists for this host with `location: MESH_INTERNAL` or specific traffic routing policies, Istio might require sidecar injection for proper mTLS handling.

**Why detection is not possible**:
- ServiceEntry resources are dynamic and can be created, modified, or deleted at any time by users or operators
- The URL detection runs during pipeline reconciliation, which happens frequently (on every pipeline change, component restart, or periodic sync)
- Querying all ServiceEntry resources on every reconciliation would:
  - Add significant performance overhead (requires API calls to list and parse ServiceEntry resources across all namespaces)
  - Create a dependency on Istio networking APIs, increasing complexity and coupling
  - Still miss edge cases where ServiceEntry configurations are ambiguous (for example, MESH_EXTERNAL with specific routing rules)

**Workaround**: If you use ServiceEntry resources to expose external services through the Istio mesh, explicitly set `export: On` in the Telemetry CR to ensure sidecar injection is enabled. The Istio sidecar correctly routes traffic based on ServiceEntry configurations, even if the URL appears external to the detection logic.

### Sidecar Injection Logic

Sidecar injection is enabled for a component if **Istio CRDs are present** AND **either** condition is met:

- **Ingestion is On** AND **component requires ingestion Istio**
- **Export is On** AND **at least one pipeline has a cluster-internal output URL**

Per-component sidecar injection decisions (all require Istio CRDs present):

- **Gateway**: Sidecar enabled if Istio CRDs present AND ((`ingestion: On`) OR (`export: On` AND at least one TracePipeline has cluster-internal output)).
- **Metric Agent**: Sidecar enabled if Istio CRDs present AND ((`ingestion: On` AND at least one MetricPipeline has `input.prometheus.enabled: true`) OR (`export: On` AND at least one MetricPipeline has cluster-internal output)).
- **Log Agents**: Sidecar enabled if Istio CRDs present AND (`export: On` AND at least one LogPipeline has cluster-internal output).

**Example 1**: If Istio CRDs not present, no components get sidecar injection regardless of configuration.

**Example 2**: If Istio CRDs present, `ingestion: Off` and `export: On`, but all pipelines use external URLs (for example, `https://external.com`), then no components get sidecar injection because there are no cluster-internal backends.

**Example 3**: If Istio CRDs present, `ingestion: On` and `export: Off`, but no MetricPipeline has `input.prometheus.enabled: true`, then the Metric Agent does not get sidecar injection because Prometheus scraping is not enabled (Gateway still gets sidecar for ingestion).

## User Examples

### Example 1: Default Behavior (Both Enabled)

Enable Istio for both ingestion and export (this is the default):

```yaml
apiVersion: operator.kyma-project.io/v1beta1
kind: Telemetry
metadata:
  name: default
  namespace: kyma-system
spec:
  istio:
    ingestion: On  # This is the default
    export: On     # This is the default
  metric:
    collectionInterval: 30s
```

Or simply omit the field (defaults to On for both):

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

**Behavior**: When both `ingestion: On` and `export: On` (or omitted), the system enables Istio integration for all components **only if Istio CRDs are present**:

- **If Istio CRDs present**:
  - **Ingestion**:
    - Gateway: Enabled (DestinationRule TLS DISABLE, sidecar injection with exclude inbound ports).
    - Metric Agent: Enabled **only if at least one MetricPipeline has `input.prometheus.enabled: true`** (mounts Istio certificates, `app-services-secure` scrape job, can scrape STRICT mTLS workloads).
    - Log Agents: No effect.
  - **Export**:
    - All components: Enabled **only for components with cluster-internal output URLs** (sidecar injection, traffic routing for mTLS backends).
- **If Istio CRDs not present**:
  - All Istio configurations are automatically skipped (safe for non-Istio clusters).

This default mode ensures maximum compatibility with Istio mesh backends when Istio is installed, and is safe for non-Istio clusters.

**Example Pipeline Configurations**:

Pipeline with Prometheus input enabled (Istio enabled for ingestion):
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
        value: http://otel-collector.observability.svc.cluster.local:4317
# Result: Metric Agent ingestion has Istio enabled with app-services-secure job (can scrape STRICT mTLS workloads)
# Result: Metric Agent export has Istio enabled (cluster-internal URL detected)
```

Pipeline without Prometheus input (Istio disabled for ingestion):
```yaml
apiVersion: telemetry.kyma-project.io/v1beta1
kind: MetricPipeline
metadata:
  name: runtime-metrics
spec:
  input:
    runtime:
      enabled: true
  output:
    otlp:
      endpoint:
        value: https://metrics.external.com:443
# Result: Metric Agent ingestion has Istio disabled (no Prometheus input)
# Result: Metric Agent export has Istio disabled (external URL detected)
```

Pipeline with cluster-internal output (Istio enabled for export):
```yaml
apiVersion: telemetry.kyma-project.io/v1beta1
kind: LogPipeline
metadata:
  name: backend-logs
spec:
  output:
    http:
      host:
        value: fluentd.logging.svc.cluster.local
      port: "8080"
# Result: Log Agent export has Istio enabled (cluster-internal URL detected)
```

### Example 2: Disable Both Ingestion and Export

Explicitly disable Istio on all ingestion and export paths:

```yaml
apiVersion: operator.kyma-project.io/v1beta1
kind: Telemetry
metadata:
  name: default
  namespace: kyma-system
spec:
  istio:
    ingestion: Off
    export: Off
  metric:
    collectionInterval: 30s
```

**Behavior**: 
- The system sets the label `sidecar.istio.io/inject` to `"false"` for all components.
- The system does not create DestinationRule resources.
- The system does not apply Istio traffic routing annotations.
- The system does not mount Istio certificate volumes.
- The system does not configure the `app-services-secure` Prometheus receiver for the metric agent.
- Components cannot communicate with STRICT mTLS backends in the Istio mesh.

**Use Case**: All pipelines send to external backends that do not require mTLS, and no workloads use STRICT mTLS policies. This saves resources by avoiding sidecar overhead.

**Example Pipeline Configuration**:

Even with cluster-internal outputs, Istio is disabled:

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
        value: http://otel-collector.observability.svc.cluster.local:4317
# Result: Metric Agent has sidecar.istio.io/inject: "false"
# Result: app-services-secure receiver is removed - cannot scrape STRICT mTLS workloads
# Result: Cannot send to backend if it requires STRICT mTLS
```

### Example 3: Enable Ingestion Only

Enable Istio for ingestion while disabling export:

```yaml
apiVersion: operator.kyma-project.io/v1beta1
kind: Telemetry
metadata:
  name: default
  namespace: kyma-system
spec:
  istio:
    ingestion: On
    export: Off
  metric:
    collectionInterval: 30s
```

**Behavior**:
- **Ingestion**:
  - Gateway: Enabled (DestinationRule TLS DISABLE).
  - Metric Agent: Enabled only if Prometheus input is enabled (mounts Istio certificates, `app-services` + `app-services-secure` receivers, can scrape STRICT mTLS workloads).
- **Export**:
  - All components: Disabled (sidecar injection disabled, cannot send to STRICT mTLS backends).

**Use Case**: Your applications use STRICT mTLS policies (need ingestion mTLS), but all backends are external and do not require mTLS (export can be plain-text). This saves resources on export sidecars while maintaining ability to scrape mTLS workloads.

**Example Pipeline Configuration**:

Pipeline with Prometheus input (Istio ingestion enabled):
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
        value: https://metrics.external.com:443
# Result: Metric Agent ingestion has Istio enabled (Prometheus input detected)
# Result: Configures both app-services and app-services-secure receivers
# Result: Metric Agent export has Istio disabled (external URL)
```

### Example 4: Disable Ingestion with Prometheus Input (Istio Proxy Metrics Only)

Disable Istio for ingestion but still collect Istio proxy metrics:

```yaml
apiVersion: operator.kyma-project.io/v1beta1
kind: Telemetry
metadata:
  name: default
  namespace: kyma-system
spec:
  istio:
    ingestion: Off
    export: Off
  metric:
    collectionInterval: 30s
```

**Behavior**:
- **Ingestion**:
  - Gateway: Disabled (no DestinationRule).
  - Metric Agent: Disabled for STRICT mTLS scraping (no Istio certificates, no `app-services-secure` receiver).
- **Export**:
  - All components: Disabled (no sidecar injection).

**Use Case**: Your applications do not use STRICT mTLS policies (don't need certificate-based scraping), and you don't need Istio sidecar integration. This saves resources by not mounting certificates or injecting sidecars.

**Example Pipeline Configuration**:

Pipeline with Prometheus input enabled:
```yaml
apiVersion: telemetry.kyma-project.io/v1beta1
kind: MetricPipeline
metadata:
  name: istio-proxy-metrics
spec:
  input:
    prometheus:
      enabled: true
  output:
    otlp:
      endpoint:
        value: https://metrics.external.com:443
# Result: Metric Agent ingestion has Istio disabled (no certificates, no app-services-secure)
# Result: Cannot scrape STRICT mTLS workloads (no app-services-secure receiver)
# Result: Cannot scrape STRICT mTLS workloads (no app-services-secure receiver)
```

### Example 5: Enable Export Only with URL Detection
```
    export: On
  metric:
    collectionInterval: 30s
```

**Behavior**:
- **Ingestion**:
  - Gateway: Disabled (no DestinationRule).
  - Metric Agent: Disabled (no Istio certificates, no `app-services-secure` job, cannot scrape STRICT mTLS workloads).
- **Export**:
  - All components: Enabled only if they have pipelines with cluster-internal output URLs (sidecar injection enabled, traffic routing for mTLS backend communication) if Istio is present.

**Use Case**: Your applications do not use STRICT mTLS policies (ingestion does not need Istio), but your backends are in the Istio mesh and require mTLS (export needs Istio sidecar). Common when using external observability platforms with in-cluster aggregators that have STRICT mTLS.

**Example Pipeline Configurations**:

Pipeline with cluster-internal output (Istio enabled for export):
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
# Result: Metric Agent export has Istio enabled (cluster-internal URL detected)
# Result: Sidecar injection enabled, traffic to port 4317 routed through Istio sidecar
```

Pipeline with external output (Istio disabled for export):
```yaml
apiVersion: telemetry.kyma-project.io/v1beta1
kind: MetricPipeline
metadata:
  name: external-backend
spec:
  output:
    otlp:
      endpoint:
        value: https://metrics.external.com:443
# Result: Metric Agent export has Istio disabled (external URL detected)
# Result: No sidecar injection for this pipeline (traffic bypasses Istio)
```

Mixed pipelines (Istio enabled if at least one is cluster-internal):
```yaml
apiVersion: telemetry.kyma-project.io/v1beta1
kind: MetricPipeline
metadata:
  name: mixed-pipeline-1
spec:
  output:
    otlp:
      endpoint:
        value: http://otel-collector.observability.svc.cluster.local:4317
---
apiVersion: telemetry.kyma-project.io/v1beta1
kind: MetricPipeline
metadata:
  name: mixed-pipeline-2
spec:
  output:
    otlp:
      endpoint:
        value: https://metrics.external.com:443
# Result: Metric Agent export has Istio enabled (at least one pipeline has cluster-internal URL)
# Result: Sidecar injection enabled, traffic to port 4317 routed through Istio, traffic to external.com bypasses Istio
```
### Implementation Impact

When `istio` fields are set, the reconciliation logic changes as follows:

#### Istio CRD Detection

Both `ingestion: On` and `export: On` modes first check for Istio presence before applying any Istio configurations:

1. **Istio CRD Check**: Scan for Istio CRDs (`*.istio.io`) in the cluster at reconciliation time.
2. **If Istio CRDs present**: Proceed with mode-specific logic (ingestion/export) and apply Istio configurations as needed.
3. **If Istio CRDs not present**: Skip all Istio configurations (no DestinationRule, no sidecar injection, no certificate mounts) regardless of mode settings.

This automatic detection ensures that `On` mode is safe for both Istio-enabled and non-Istio clusters, and prevents configuration of unnecessary Istio resources when Istio is not installed.

#### Prometheus Input Detection for Metric Agent Ingestion

When `ingestion: On` **and Istio CRDs are present**, the Metric Agent checks if Istio is needed:

1. **Pipeline Analysis**: Scan all active MetricPipelines for `input.prometheus.enabled: true`.
2. **Decision**: If **at least one** MetricPipeline has `input.prometheus.enabled: true`, enable Istio for ingestion.
3. **If no Prometheus input enabled**: Skip Istio ingestion configurations for the Metric Agent (no certificates, no `app-services-secure` job).

This optimization ensures Istio certificates and the `app-services-secure` job are only configured when Istio is installed AND actually needed for scraping STRICT mTLS workloads.

#### Cluster-Internal URL Detection for Export

When `export: On`, the system performs URL analysis for each pipeline:

1. **URL Extraction**: Extract output URLs from all active pipelines:
   - TracePipelines: `output.otlp.endpoint`
   - MetricPipelines: `output.otlp.endpoint`
   - LogPipelines: `output.http.host`

2. **URL Classification**: Determine if the URL is cluster-internal or external:
   - **Cluster-internal patterns**:
     - Kubernetes service DNS: `*.*.svc.cluster.local`, `*.*.svc`, `*.*`, or single-label names
     - Kubernetes ClusterIP addresses (within service CIDR)
   - **External patterns**:
     - Fully qualified external domains (for example, `logs.external.com`)
     - Public IP addresses
     - Loopback addresses (for example, `localhost`, `127.0.0.1`)

3. **Per-Component Decision**: For each component, check its associated pipelines:
   - **Gateway**: If ANY TracePipeline has a cluster-internal output → enable Istio export
   - **Metric Agent**: If ANY MetricPipeline has a cluster-internal output → enable Istio export
   - **Log Agents**: If ANY LogPipeline has a cluster-internal output → enable Istio export

#### Sidecar Injection Decision Logic

Sidecar injection is enabled for a component if **either** condition is met (and Istio CRDs are present):

- **Ingestion is On** AND **component requires ingestion Istio**
- **Export is On** AND **at least one pipeline has a cluster-internal output URL**

Per-component sidecar injection decisions:

- **Gateway**: Sidecar enabled if (`ingestion: On`) OR (`export: On` AND at least one TracePipeline has cluster-internal output).
- **Metric Agent**: Sidecar enabled if (`ingestion: On` AND at least one MetricPipeline has `input.prometheus.enabled: true`) OR (`export: On` AND at least one MetricPipeline has cluster-internal output).
- **Log Agents**: Sidecar enabled if (`export: On` AND at least one LogPipeline has cluster-internal output).

**Example 1**: If `ingestion: Off` and `export: On`, but all pipelines use external URLs (for example, `https://external.com`), then no components get sidecar injection because there are no cluster-internal backends.

**Example 2**: If `ingestion: On` and `export: Off`, but no MetricPipeline has `input.prometheus.enabled: true`, then the Metric Agent does not get sidecar injection because Prometheus scraping is not enabled (Gateway still gets sidecar for ingestion).

**Example 3**: If `ingestion: Off` and `export: On`, and one MetricPipeline uses `http://otel-collector.observability.svc.cluster.local:4317`, then the Metric Agent gets sidecar injection with traffic routing annotations for that backend port.

#### Traffic Routing Annotations for Export

When a component enables Istio for export (cluster-internal URLs detected), the following annotations are applied:

1. **OTLP Gateway**:
   - `traffic.sidecar.istio.io/includeOutboundPorts`: Set to the list of backend ports extracted from TracePipeline outputs with cluster-internal URLs (for example, `"4317,4318"`).
   - This ensures traffic to cluster-internal backends goes through the Istio sidecar for mTLS.

2. **Metric Agent**:
   - `traffic.sidecar.istio.io/includeOutboundPorts`: Set to the list of backend ports extracted from MetricPipeline outputs with cluster-internal URLs (for example, `"4317,9090"`).
   - This ensures traffic to cluster-internal backends goes through the Istio sidecar for mTLS.

3. **OTel Log Agent**:
   - `traffic.sidecar.istio.io/includeOutboundPorts`: Set to the list of backend ports extracted from LogPipeline outputs with cluster-internal URLs (for example, `"8080,24224"`).
   - This ensures traffic to cluster-internal backends goes through the Istio sidecar for mTLS.

4. **Fluent Bit**:
   - `traffic.sidecar.istio.io/includeOutboundPorts`: Set to the list of backend ports extracted from LogPipeline outputs with cluster-internal URLs (for example, `"8080,24224"`).
   - This ensures traffic to cluster-internal backends goes through the Istio sidecar for mTLS.

**Important**: Only ports for cluster-internal URLs are included. External URLs bypass the Istio sidecar.

#### Istio Processor Configuration

**Critical**: The `istio_enrichment` and `istio_noise_filter` processor configurations are **independent** from the `ingestion` and `export` mode settings. All OTel workloads (Gateway, Metric Agent, Log Agents) must **always include** these processors in their pipelines, regardless of Istio mode configuration.

**Configuration per component**:

1. **OTLP Gateway**:
   - Always configure `istio_enrichment` processor in trace, metric, and log pipelines.
   - Always configure `istio_noise_filter` processor in trace, metric, and log pipelines.
   - Independent of `ingestion` and `export` settings.
   - Enriches telemetry with Istio service mesh metadata (workload identity, mesh attributes).
   - Filters out noisy Istio-related telemetry (health checks, internal communication).

2. **Metric Agent**:
   - Always configure `istio_enrichment` processor in metric pipelines.
   - Always configure `istio_noise_filter` processor in metric pipelines.
   - Independent of `ingestion` and `export` settings.
   - Enriches metrics with Istio service mesh metadata.
   - Filters out noisy Istio-related metrics.

3. **OTel Log Agent**:
   - Always configure `istio_enrichment` processor in log pipelines.
   - Always configure `istio_noise_filter` processor in log pipelines.
   - Independent of `ingestion` and `export` settings.
   - Enriches logs with Istio service mesh metadata.
   - Filters out noisy Istio-related logs.

4. **Fluent Bit**:
   - Fluent Bit does not support OTel processors, so neither `istio_enrichment` nor `istio_noise_filter` are applicable.

**Rationale**: These processors provide valuable context about the service mesh environment and reduce telemetry noise, which is useful for telemetry correlation and cost optimization. They operate on telemetry data within the OTel pipeline and do not require Istio certificates or sidecar injection.

#### Components Affected

All reconcilers that create or configure telemetry components must respect the `istio` settings:

1. **OTLP Gateway Reconciler**
   - **Ingestion On**: Check for Istio CRDs. If present, apply DestinationRule (TLS DISABLE), sidecar injection with `includeInboundPorts: ""` annotation. If not present, skip all Istio configurations.
   - **Ingestion Off**: Skip DestinationRule.
   - **Export On**: 
     1. Check for Istio CRDs. If not present, skip.
     2. Scan all TracePipeline `output.otlp.endpoint` values.
     3. If at least one has a cluster-internal URL:
        - Enable sidecar injection (`sidecar.istio.io/inject: "true"`).
        - Apply `traffic.sidecar.istio.io/includeOutboundPorts` with backend ports from cluster-internal URLs.
     4. If all URLs are external, skip sidecar injection (if ingestion is also Off).
   - **Export Off**: Set sidecar injection label to `"false"` (if ingestion is also Off).
   - **Always**: Configure `istio_enrichment` and `istio_noise_filter` processors in all trace, metric, and log pipelines (independent of ingestion/export settings).

2. **Metric Agent Reconciler**
   - **Ingestion On**: 
     1. Check for Istio CRDs. If not present, skip all Istio ingestion configurations.
     2. Scan all MetricPipelines for `input.prometheus.enabled: true`.
     3. If at least one MetricPipeline has Prometheus input enabled:
        - Mount Istio certificates (volume mount at `/etc/istio-output-certs`).
        - Configure `app-services` Prometheus receiver (scrapes application services without mTLS).
        - Configure `app-services-secure` Prometheus receiver (scrapes STRICT mTLS workloads with Istio certificates).
        - Apply `proxy.istio.io/config` annotation (write certificates to shared volume).
        - Apply `sidecar.istio.io/userVolumeMount` annotation (mount certificate volume into sidecar).
        - Enable sidecar injection (`sidecar.istio.io/inject: "true"`).
     4. If no MetricPipeline has Prometheus input enabled, skip Istio ingestion configurations.
   - **Ingestion Off**: 
     1. Skip Istio certificate volume mounts.
     2. Remove `app-services-secure` Prometheus receiver.
     3. Configure `app-services` Prometheus receiver without Istio certificate support.
   - **Export On**: 
     1. Check for Istio CRDs. If not present, skip.
     2. Scan all MetricPipeline `output.otlp.endpoint` values.
     3. If at least one has a cluster-internal URL:
        - Enable sidecar injection (`sidecar.istio.io/inject: "true"`).
        - Apply `traffic.sidecar.istio.io/includeOutboundPorts` with backend ports from cluster-internal URLs.
     4. If all URLs are external, skip sidecar injection (if ingestion is also Off or no Prometheus input enabled).
   - **Export Off**: Set sidecar injection label to `"false"` (if ingestion is also Off or no Prometheus input enabled).
   - **Always**: Configure `istio_enrichment` and `istio_noise_filter` processors in all metric pipelines (independent of ingestion/export settings).

3. **OTel Log Agent Reconciler**
   - **Export On**: 
     1. Check for Istio CRDs. If not present, skip.
     2. Scan all LogPipeline `output.http.host` values.
     3. If at least one has a cluster-internal URL:
        - Enable sidecar injection (`sidecar.istio.io/inject: "true"`).
        - Apply `traffic.sidecar.istio.io/includeOutboundPorts` with backend ports from cluster-internal URLs.
     4. If all URLs are external, skip sidecar injection.
   - **Export Off**: Set sidecar injection label to `"false"`.
   - **Always**: Configure `istio_enrichment` and `istio_noise_filter` processors in all log pipelines (independent of export setting).

4. **Fluent Bit Reconciler**
   - **Export On**: 
     1. Check for Istio CRDs. If not present, skip.
     2. Scan all LogPipeline `output.http.host` values.
     3. If at least one has a cluster-internal URL:
        - Enable sidecar injection (`sidecar.istio.io/inject: "true"`).
        - Apply `traffic.sidecar.istio.io/includeOutboundPorts` with backend ports from cluster-internal URLs.
     4. If all URLs are external, skip sidecar injection.
   - **Export Off**: Set sidecar injection label to `"false"`.
   - **Note**: Fluent Bit does not support OTel processors, so neither `istio_enrichment` nor `istio_noise_filter` are applicable.

5. **NetworkPolicy Reconcilers**
   - Include Istio Envoy port (15090) in ingress rules only for components where sidecar injection is enabled.
   - Sidecar injection is enabled when:
     - Istio CRDs are present AND
     - (`ingestion: On` OR (`export: On` AND at least one pipeline has cluster-internal output))
   - **Off**: Omit Istio Envoy port from all ingress rules.

### Migration Path

The proposed API provides a smooth two-phase migration path to eventually reach an "Istio mode Off by default" model while maintaining backward compatibility during the transition.

#### Phase 1: Introduce Global Ingestion/Export API with On Default

- Add `istio.ingestion` and `istio.export` fields with simple mode controls (On | Off).
- Default both fields to `On` to ensure backward compatibility with existing behavior.
- Ingestion and export can be configured independently to optimize resource usage when needed.
- Users can explicitly opt out by setting fields to `Off`.

**Pros:**
- **Simplest possible API**: Two boolean-like fields (On/Off) control all components.
- **Clear separation of concerns**: Ingestion (receiving/scraping data) vs export (sending data to backends) are distinct concepts that map well to Istio requirements.
- **Intelligent export mode**: When `export: On`, the system analyzes pipeline URLs and only enables Istio for components with cluster-internal backends, automatically optimizing resource usage.
- **Backward compatibility**: Default `On` ensures existing behavior is preserved.
- **Flexible control**: Users can disable ingestion and/or export independently to save resources when not needed.
- **Easy to understand**: Explicit On/Off modes with automatic URL-based optimization for export.
- **Low complexity**: URL detection is straightforward pattern matching; no complex pipeline state tracking needed.

**Cons:**
- **Global ingestion mode**: Cannot selectively enable ingestion Istio for only some components (all or nothing for ingestion).
- **URL-based heuristic**: Cluster-internal URL detection might not capture all cases (for example, NodePort services accessed using node IP).
- **Global application**: All components share the same ingestion and export settings (cannot enable Istio for only Gateway while disabling for Metric Agent).
- **Not safe for non-Istio clusters by default**: Phase 1 defaults to `On`, which configures Istio resources even when Istio is not installed. Users without Istio must explicitly set to `Off`.

#### Phase 2: Change Default to Off (Explicit Opt-In)

After Phase 1 has been stable and users have had time to explicitly configure their Istio integration preferences, transition to an opt-in model:

- Change the default value for `istio.ingestion` from `On` to `Off`.
- Change the default value for `istio.export` from `On` to `Off`.
- Users who need Istio integration must explicitly set `ingestion: On` and/or `export: On` in their Telemetry CR.
- This change addresses [issue #657](https://github.com/kyma-project/telemetry-manager/issues/657) by making Istio mode opt-in by default.

**Migration Strategy for Phase 2**:

1. **Announce deprecation**: Provide advance notice (at least 2 releases) that the default will change to `Off`.
2. **Audit tooling**: Provide a command or script that checks existing clusters for Istio usage and generates recommended Telemetry CR configurations.
3. **Automatic migration**: On upgrade, if the cluster has Istio CRDs present and the Telemetry CR does not explicitly set `istio` fields, add `ingestion: On, export: On` to preserve behavior.
4. **Documentation**: Update all examples and guides to show explicit Istio configuration.

**Phase 2 Benefits**:
- **Resource efficiency**: Clusters without Istio requirements do not pay the overhead of sidecar containers and Istio configurations.
- **Explicit configuration**: Users consciously decide when to enable Istio integration, reducing surprise behavior.
- **Cleaner defaults**: New clusters start with minimal overhead and add Istio integration only when needed.

**Phase 2 Breaking Changes**:
- Existing Telemetry CRs without explicit `istio` configuration will change behavior on upgrade (Istio disabled by default).
- Mitigation: Automatic migration adds explicit `istio: {ingestion: On, export: On}` to existing CRs during upgrade when Istio CRDs are detected.

**Timeline**:
- **Phase 1**: Immediate implementation (current proposal).
- **Phase 2**: After Phase 1 has been stable for at least 2 releases and users have adopted explicit configuration.

### Backward Compatibility

#### Phase 1: On by Default

The global ingestion/export API approach ensures backward compatibility during Phase 1:

- **Default `On` for both ingestion and export**: Preserves current behavior where Istio integration is enabled for all components.
- **No breaking changes**: Existing Telemetry CRs without the `istio` field work with the default `On` setting, which matches current implicit behavior.
- **Explicit control when needed**: Users can set `ingestion: Off` and/or `export: Off` to disable Istio and save resources when not needed.

**Comparison with Current Behavior**:

| Current (Implicit)                    | New API Equivalent                    | Behavior                                                |
|---------------------------------------|---------------------------------------|---------------------------------------------------------|
| Always enabled (when Istio detected)  | `ingestion: On, export: On` (default) | Full Istio integration for all components and all paths |
| User wants to disable                 | `ingestion: Off, export: Off`         | All Istio disabled, opt-out for resource savings        |

**Migration Recommendation for Existing Clusters**:

For clusters with Istio:
- **No action needed**: The default `ingestion: On, export: On` preserves current behavior.
- **Optional optimization**: Users can explicitly set `ingestion: Off` and/or `export: Off` if they want to save resources and do not need Istio integration.

For clusters without Istio:
- **Action needed**: Set `istio: {ingestion: Off, export: Off}` to avoid configuring unnecessary Istio resources.
- **Why**: Without Istio installed, enabling Istio mode will configure resources (DestinationRule, sidecar injection labels) that have no effect but add configuration overhead.

For users migrating to Istio:
- **Action needed**: Set `istio: {ingestion: On, export: On}` after installing Istio to enable integration.
- **Flexible control**: Users can enable/disable ingestion and export independently (for example, `ingestion: Off, export: On`).

#### Phase 2: Off by Default (Future State)

When Phase 2 is implemented, the backward compatibility approach changes:

- **Default `Off` for both ingestion and export**: New Telemetry CRs without explicit `istio` configuration have Istio integration disabled.
- **Automatic migration for existing CRs**: During upgrade, existing Telemetry CRs without explicit `istio` fields automatically receive `istio: {ingestion: On, export: On}` to preserve current behavior (based on current enabled state, not Istio CRD detection).
- **Explicit opt-in required**: New clusters or new Telemetry CRs must explicitly set `ingestion: On` and/or `export: On` to enable Istio integration.

**Comparison with Phase 1 Behavior**:

| Scenario                              | Phase 1 (On Default)                  | Phase 2 (Off Default)                                                 |
|---------------------------------------|---------------------------------------|-----------------------------------------------------------------------|
| New Telemetry CR, no `istio` field   | `ingestion: On, export: On` (default) | `ingestion: Off, export: Off` (default)                               |
| Existing Telemetry CR during upgrade | No change                             | Automatic migration adds `ingestion: On, export: On`                  |
| Explicit `istio: {ingestion: On}`    | Behavior unchanged                    | Behavior unchanged                                                    |

**Migration Recommendation for Phase 2 Transition**:

Before Phase 2 is deployed:
1. **Audit your Telemetry CRs**: Check if your cluster has Istio and whether your pipelines require Istio integration.
2. **Explicitly configure**: Add `istio: {ingestion: On, export: On}` to your Telemetry CR if you need Istio integration.
3. **Test opt-out**: If you currently have Istio but don't need the integration, test with `istio: {ingestion: Off, export: Off}` before Phase 2 arrives.

During Phase 2 upgrade:
- The telemetry-manager upgrade automatically adds explicit `istio` configuration to existing CRs when Istio CRDs are detected.
- Review the automated migration and adjust if needed (for example, you may want to disable Istio after the upgrade).
