---
title: Istio Mode Configuration API
status: Proposed
date: 2026-06-10
---

# Istio Mode Configuration API

## Context and Problem Statement

Telemetry Manager currently auto-detects Istio by checking for Istio CRDs (`*.istio.io`) in the cluster and automatically applies Istio-specific resources when detected. This includes sidecar injection labels, PeerAuthentication resources, DestinationRules, traffic routing annotations, and certificate volume mounts across telemetry components (OTLP Gateway, Metric Agent, OTel Log Agent, Fluent Bit, and Self-Monitor).

While automatic detection provides convenience, it creates several operational challenges:

- No explicit control: Users cannot disable Istio mode even if they do not need it, resulting in unnecessary resource overhead from sidecar containers, certificate management, and Istio-specific network policies.

- Unconditional artifacts: Some Istio-related behaviors remain applied unconditionally regardless of detection. For example, all components (except Self-Monitor) always have `sidecar.istio.io/inject: "true"` and the Metric Agent always mounts Istio certificate volumes, even when Istio integration is not required.

- All-or-nothing approach: The current design does not allow granular control. Users cannot selectively enable Istio mode for specific components or use cases (such as backends requiring in-cluster mTLS).

- Migration difficulty: To move toward an "Istio mode OFF by default" model (tracked in [issue #657](https://github.com/kyma-project/telemetry-manager/issues/657)), the system requires a backward-compatible transition path that preserves existing behavior while allowing explicit opt-in configuration.

The system needs an API mechanism to explicitly enable or disable Istio mode, providing users with control over when and how Istio integration is applied to telemetry components. This addresses [issue #3549](https://github.com/kyma-project/telemetry-manager/issues/3549), which proposes an explicit configuration model that supports eventual migration to an opt-in default while maintaining backward compatibility during the transition.

## Current Istio Integration

### Istio Detection

The Telemetry Manager automatically detects Istio by checking for the presence of Istio CRDs (Custom Resource Definitions) in the cluster. Specifically, it looks for API groups matching `*.istio.io`.

### Component Behaviors

#### OTLP Gateway

The OTLP Gateway is deployed as a DaemonSet and serves as the unified ingress point for all telemetry signals (logs, traces, and metrics).

##### Unconditional (Always Applied)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Label | `sidecar.istio.io/inject: "true"` | The OTLP Gateway always has Istio sidecar injection enabled to support outbound mTLS communication to in-cluster backends in the Istio mesh. |

##### Conditional (Only When Istio is Detected)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| PeerAuthentication | PERMISSIVE mTLS mode | This configuration allows both plain-text and mTLS connections to the OTLP Gateway. This ensures applications can send data over plain-text (because of node-local ingestion), while still supporting mTLS for any direct cross-namespace access if needed. |
| DestinationRule | `TLS mode: DISABLE` for all OTLP Services | The OTLP Gateway receives telemetry data over plain-text on the ingestion path because of node-local routing. Disabling TLS for client connections to these services ensures that other components (such as metric pipelines) can connect without requiring mTLS. |
| NetworkPolicy (Ingress) | Additionally allows traffic on Istio Envoy telemetry port (15090) | When Istio is present, the sidecar's Envoy proxy exposes metrics that need to be scraped by monitoring systems. |

#### Metric Agent

The Metric Agent is deployed as a DaemonSet and scrapes Prometheus metrics from workloads in the cluster, then forwards them to configured backends.

##### Unconditional (Always Applied)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Label | `sidecar.istio.io/inject: "true"` | The Metric Agent always has Istio sidecar injection enabled to support outbound mTLS communication to in-cluster backends in the Istio mesh. |

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


#### OTel Log Agent

The OTel Log Agent is deployed as a DaemonSet and collects container logs using file-based collection, then forwards them to configured backends.

##### Unconditional (Always Applied)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Label | `sidecar.istio.io/inject: "true"` | The OTel Log Agent always has Istio sidecar injection enabled to support outbound mTLS communication to in-cluster backends in the Istio mesh. |

##### Conditional (Only When Istio is Detected)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| NetworkPolicy (Ingress) | Additionally allows traffic on Istio Envoy telemetry port (15090) | When Istio is present, the sidecar's Envoy proxy exposes metrics that need to be scraped by monitoring systems. |

#### Fluent Bit

Fluent Bit is deployed as a DaemonSet and provides legacy log collection capabilities.

##### Unconditional (Always Applied)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| Pod Label | `sidecar.istio.io/inject: "true"` | Fluent Bit always has Istio sidecar injection enabled to support outbound mTLS communication to in-cluster backends in the Istio mesh. |

##### Conditional (Only When Istio is Detected)

| Resource | Behavior | Rationale |
|----------|----------|-----------|
| NetworkPolicy (Ingress) | Additionally allows traffic on Istio Envoy telemetry port (15090) | When Istio is present, the sidecar's Envoy proxy exposes metrics that need to be scraped by monitoring systems. |

#### Self-Monitor

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
| OTLP Gateway | Always enabled | Not used | None | PeerAuthentication (PERMISSIVE), DestinationRule (TLS DISABLE) |
| Metric Agent | Always enabled | Always mounted | Conditional (traffic routing) | None |
| OTel Log Agent | Always enabled | Not used | None | None |
| Fluent Bit | Always enabled | Not used | None | None |
| Self-Monitor | Always disabled | Not used | None | None |


## Proposed API

To provide explicit control over Istio integration, we propose adding an `istio` field to the Telemetry CR spec with an enum-based mode configuration. This field allows users to control Istio mode globally for all telemetry components.

### API Schema

```yaml
spec:
  istio:
    mode: <AUTO | OFF>  # Default: AUTO
```

- **AUTO**: Automatically detect whether Istio integration is needed based on:
  - Presence of Istio CRDs (`*.istio.io`) in the cluster
  - Pipeline configurations that require Istio (for example, Metric Agent with Istio input, Prometheus input scraping Istio metrics)
  - When both Istio is present AND pipelines need it, apply Istio-specific resources
- **OFF**: Disable Istio integration regardless of whether Istio is present in the cluster or pipelines require it

## User Examples

### Example 1: Auto-detection (Default Behavior)

Automatically detect Istio and apply integration:

```yaml
apiVersion: operator.kyma-project.io/v1beta1
kind: Telemetry
metadata:
  name: default
  namespace: kyma-system
spec:
  istio:
    mode: AUTO  # This is the default
  metric:
    collectionInterval: 30s
```

Or simply omit the field (defaults to AUTO):

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

**Behavior**: When `mode: AUTO` (or omitted), the system intelligently detects whether Istio integration is needed:

1. **Istio CRD Detection**: Checks if Istio CRDs (`*.istio.io`) are present in the cluster
2. **Pipeline Analysis**: Analyzes pipeline configurations to determine if they require Istio:
   - MetricPipelines with Istio input enabled
   - MetricPipelines with Prometheus input scraping Istio metrics
   - Pipelines targeting in-cluster backends that require mTLS
3. **Smart Activation**: Applies Istio-specific resources (sidecar injection, PeerAuthentication, DestinationRule, traffic annotations, certificate volumes) only when BOTH conditions are met:
   - Istio is present in the cluster
   - At least one pipeline requires Istio integration

If Istio is not present or no pipelines require it, no Istio-specific configurations are applied.

### Example 2: Explicit Disable (Opt-out)

Force Istio mode off, even if Istio is detected in the cluster:

```yaml
apiVersion: operator.kyma-project.io/v1beta1
kind: Telemetry
metadata:
  name: default
  namespace: kyma-system
spec:
  istio:
    mode: OFF
  metric:
    collectionInterval: 30s
```

**Behavior**: 
- No sidecar injection labels (`sidecar.istio.io/inject` set to `"false"`).
- No PeerAuthentication or DestinationRule resources created.
- No Istio traffic routing annotations.
- No Istio certificate volume mounts.
- Metric Agent cannot scrape STRICT mTLS workloads (they would need PERMISSIVE mode).
- Istio metric scraping remains possible if Istio is present in the cluster.

### Implementation Impact

When `istio.mode` is set, the reconciliation logic changes as follows:

#### Detection Logic for AUTO Mode

When `mode: AUTO`, the system performs intelligent detection:

1. **Istio CRD Check**: Scan for Istio CRDs (`*.istio.io`) in the cluster
2. **Pipeline Analysis**: Check if any active pipelines require Istio:
   - MetricPipelines with `input.istio.enabled: true`
   - MetricPipelines with `input.prometheus` scraping Istio metrics (targets with `istio` labels)
   - Any pipeline targeting in-cluster backends that are part of the Istio mesh
3. **Decision**: Enable Istio integration only if BOTH conditions are true:
   - Istio CRDs are present
   - At least one pipeline requires Istio integration

#### Components Affected

All reconcilers that create or configure telemetry components must respect the `istio.mode` setting:

1. OTLP Gateway Reconciler
   - When `mode: AUTO`: Apply Istio resources only if detection logic confirms Istio is needed
   - When `mode: OFF`: Skip PeerAuthentication creation, skip DestinationRule creation, set sidecar injection label to `false`

2. Metric Agent Reconciler
   - When `mode: AUTO`: Apply Istio configurations only if detection logic confirms Istio is needed (Istio CRDs present AND pipelines with Istio/Prometheus inputs exist)
   - When `mode: OFF`: Skip Istio traffic routing annotations, skip certificate volume mounts, set sidecar injection label to `false`

3. OTel Log Agent Reconciler
   - When `mode: AUTO`: Apply sidecar injection only if detection logic confirms Istio is needed
   - When `mode: OFF`: Set sidecar injection label to `false`

4. Fluent Bit Reconciler
   - When `mode: AUTO`: Apply sidecar injection only if detection logic confirms Istio is needed
   - When `mode: OFF`: Set sidecar injection label to `false`

5. NetworkPolicy Reconcilers
   - When `mode: AUTO`: Include Istio Envoy port (15090) in ingress rules only if detection logic confirms Istio is needed
   - When `mode: OFF`: Omit Istio Envoy port (15090) from ingress rules

### Validation

Additional validation is enforced at the admission webhook level:

- When `istio.mode: AUTO`, no additional validation is required (intelligent detection handles both Istio presence and pipeline requirements).
- When `istio.mode: OFF`, warn users if:
  - MetricPipelines are configured with Istio input enabled (`input.istio.enabled: true`)
  - MetricPipelines are configured to scrape Istio metrics via Prometheus input
  - Pipelines are configured to scrape STRICT mTLS workloads (Metric Agent won't have Istio certificates)

### Migration Path

The proposed API provides a smooth migration path:

#### Introduce Enum with AUTO Default (Current Proposal)
- Add `istio.mode` field with `AUTO` as the default value
- Users can opt out with `mode: OFF`
- `AUTO` mode intelligently detects both Istio presence AND pipeline requirements (not just Istio CRDs)
- Existing installations continue working unchanged (default `AUTO` behavior)
- The enum provides clear semantics: `AUTO` = intelligent auto-detect, `OFF` = explicitly disabled

**Pros:**
- Zero disruption during initial rollout: Default `AUTO` matches current auto-detection behavior
- Intelligent resource management: Istio integration enabled only when actually needed by pipelines
- Reduced overhead: Clusters with Istio installed but no Istio-dependent pipelines avoid unnecessary resource usage
- Clear semantics: `AUTO` vs `OFF` is more explicit than `true` vs `false` vs `nil`
- Extensible: Easy to add new mode values later if needed without breaking changes
- Backward compatible: Existing installations work unchanged with default `AUTO`
- Low risk: Enum provides type safety and clear intent

**Cons:**
- More complex detection logic: Requires analyzing pipeline configurations in addition to CRD presence
- Dynamic behavior: Istio integration may be enabled/disabled as pipelines are created/deleted
- Potential confusion: Users might not understand when AUTO enables Istio (though this is more predictable than always-on)

### Backward Compatibility

The enum-based approach with `AUTO` default ensures full backward compatibility:

- Existing Telemetry CRs without the `istio` field continue to use auto-detection (default `mode: AUTO`)
- Users who upgrade to the new API version can explicitly control Istio mode without modifying existing CRs
- The default behavior (auto-detection when unset) matches the current implementation
- The enum provides clear semantics that are self-documenting in the API
