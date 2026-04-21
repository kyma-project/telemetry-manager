---
title: Optional Runtime Metrics
status: Proposed
date: 2026-04-17
---

# Optional Runtime Metrics

## Context and Problem Statement

### What the Runtime Input Is

The MetricPipeline **runtime** input collects Kubernetes infrastructure metrics about user workloads. It is an abstraction over two OpenTelemetry Collector receivers:

- The [kubeletstatsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/kubeletstatsreceiver) runs as an agent on each Node and scrapes the Kubelet API for real-time resource usage metrics: CPU, memory, filesystem, and network statistics for Pods, containers, Nodes, and volumes.
- The [k8sclusterreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sclusterreceiver) connects to the Kubernetes API server and collects cluster-level state metrics: replica counts for Deployments, DaemonSets, StatefulSets, and Jobs; container request/limit values; Pod phase; and similar metadata-driven metrics.

Together, these receivers provide a comprehensive view of Kubernetes resource health and utilization. The runtime input abstracts over both, presenting a unified set of metrics organized by resource type.

### Current API: Resource Type Selection

The runtime input already supports per-resource-type metric selection through the `resources` section. Each Kubernetes resource type (Pod, container, Node, volume, Deployment, DaemonSet, StatefulSet, Job) can be individually enabled or disabled:

```yaml
input:
  runtime:
    enabled: true
    resources:
      pod:
        enabled: true
      container:
        enabled: true
      node:
        enabled: false
      volume:
        enabled: false
```

This works well because all metrics follow a clear resource type affinity: every metric name starts with a resource prefix (such as `k8s.pod.*`, `k8s.node.*`, `k8s.container.*`), and users typically reason about observability in terms of resource types. The resource type is a strong, natural categorization.

### How the Upstream Receivers Expose Metric Selection

Both receivers offer metric selection, but through different mechanisms:

The **kubeletstats receiver** provides two levels of control:

- **Metric groups**: A `metric_groups` list that enables or disables entire resource categories (`container`, `pod`, `node`, `volume`). This is the coarse-grained toggle — the runtime input's `resources` section maps directly to this mechanism.
- **Per-metric toggles**: A `metrics` map where each metric name can be individually set to `enabled: true` or `enabled: false`. The receiver defines a default enabled/disabled state for each metric. Currently, the operator hardcodes specific overrides here (for example, enabling `container.cpu.usage` and `k8s.pod.cpu.usage`, disabling `k8s.node.cpu.time` and `k8s.node.memory.page_faults`).

The **k8s_cluster receiver** provides:

- **Per-metric toggles**: A `metrics` map, identical in concept to kubeletstats. The operator uses this to disable metrics it considers outside the curated set (for example, `k8s.container.storage_request`, `k8s.namespace.phase`, all HPA and ReplicaSet metrics).
- **Config-driven metric families**: Special configuration options like `node_conditions_to_report` and `allocatable_types_to_report` that control entire families of dynamically named metrics. These do not follow the standard per-metric toggle pattern and are out of scope for this ADR.

Both receivers use the same underlying mechanism for optional metrics: each metric has a default state (enabled or disabled) defined in the receiver metadata, and the operator can override any of them. The optional metrics discussed in this ADR are those that both receivers define as disabled by default.

### The Problem: Default Set vs Optional Metrics

Within each resource type, the runtime input exposes only a curated subset of the metrics available from the upstream receivers. This minimal default set is an intentional design decision driven by backend storage costs — every additional metric creates time series that users pay for at the backend (for example, SAP Cloud Logging). The full list of default metrics per resource type is documented in [Runtime Metrics](../../user/collecting-metrics/runtime-metrics.md).

Both upstream receivers define additional metrics that are disabled by default. Users have requested access to some of these metrics — in particular, CPU and memory utilization ratios relative to requests and limits (see [#3336](https://github.com/kyma-project/telemetry-manager/issues/3336)). Today, computing these ratios requires complex calculations in downstream systems (for example, joining usage metrics with limit metrics and computing the ratio in query language), which adds significant operational overhead.

This ADR proposes an API extension that gives users explicit opt-in control over individual optional metrics, without changing the default behavior for existing users.

## Background: Upstream Optional Metrics

### Upstream Receiver Stability

Both receivers are at **beta** stability for metrics. Individual optional metrics are at **development** stability, meaning their names or semantics can change without a formal deprecation cycle. However, beta-level receivers generally provide a deprecation notice before breaking changes.

### kubeletstats Receiver: Optional Metrics

The following metrics are disabled by default in the kubeletstats receiver:

| Resource  | Metric                                    | Description                                        |
|-----------|-------------------------------------------|----------------------------------------------------|
| Pod       | `k8s.pod.cpu_request_utilization`         | CPU usage as a ratio of container requests          |
| Pod       | `k8s.pod.cpu_limit_utilization`           | CPU usage as a ratio of container limits            |
| Pod       | `k8s.pod.cpu.node.utilization`            | CPU usage as a ratio of node capacity               |
| Pod       | `k8s.pod.memory_request_utilization`      | Memory usage as a ratio of container requests       |
| Pod       | `k8s.pod.memory_limit_utilization`        | Memory usage as a ratio of container limits         |
| Pod       | `k8s.pod.memory.node.utilization`         | Memory usage as a ratio of node capacity            |
| Pod       | `k8s.pod.uptime`                          | Time the Pod has been running                       |
| Pod       | `k8s.pod.volume.usage`                    | Bytes used in the Pod volume                        |
| Container | `k8s.container.cpu_request_utilization`   | CPU usage as a ratio of container requests          |
| Container | `k8s.container.cpu_limit_utilization`     | CPU usage as a ratio of container limits            |
| Container | `k8s.container.cpu.node.utilization`      | CPU usage as a ratio of node capacity               |
| Container | `k8s.container.memory_request_utilization`| Memory usage as a ratio of container requests       |
| Container | `k8s.container.memory_limit_utilization`  | Memory usage as a ratio of container limits         |
| Container | `k8s.container.memory.node.utilization`   | Memory usage as a ratio of node capacity            |
| Container | `container.uptime`                        | Time the container has been running                 |
| Node      | `k8s.node.uptime`                         | Time the Node has been running                      |

### k8s_cluster Receiver: Optional Metrics

The following metrics are disabled by default in the k8s_cluster receiver:

| Resource  | Metric                                    | Description                                        |
|-----------|-------------------------------------------|----------------------------------------------------|
| Container | `k8s.container.status.state`              | Current state: running, waiting, or terminated      |
| Container | `k8s.container.status.reason`             | Reason: CrashLoopBackOff, OOMKilled, and others    |
| Pod       | `k8s.pod.status_reason`                   | Status reason: Evicted, NodeLost, Shutdown, and others |
| Node      | `k8s.node.condition`                      | Condition: Ready, MemoryPressure, DiskPressure, PIDPressure |

> [!NOTE]
> The k8s_cluster receiver also has config-driven metric families (`k8s.node.allocatable_*`, `k8s.node.condition_*`) that use a different configuration mechanism than the standard per-metric toggle. These are out of scope for this ADR.

## API Location: MetricPipeline vs Telemetry CR

The optional metrics configuration belongs in the **MetricPipeline** CR, not the Telemetry CR. The existing `resources` section in the runtime input already provides per-resource-type metric selection at the MetricPipeline level. Adding optional metric selection in the same location keeps the API consistent: all decisions about which runtime metrics to collect live in one place.

## Considered Alternatives

### Option A: Metric Set Enum Per Resource

Add a `metricSet` enum field (`default` | `all`) to each resource configuration:

```yaml
resources:
  pod:
    enabled: true
    metricSet: all
```

**Pros:**
- Minimal API surface — one field per resource type
- Easy to understand and document

**Cons:**
- Too coarse-grained: `all` enables every optional metric for the resource type, including metrics the user does not need; this contradicts the storage cost motivation for the curated default set
- New optional metrics added in future upstream versions are silently included, potentially increasing costs on collector version upgrades

### Option B: Named Metric Groups Per Resource

Add a `metricGroups` list field that references curated, named groups of metrics:

```yaml
resources:
  pod:
    enabled: true
    metricGroups:
      - utilization
      - uptime
```

**Pros:**
- More granular than a single boolean or enum
- Curated groups can be documented with cost impact

**Cons:**

The core problem with this approach is that it introduces a second layer of categorization below resource type, and that layer is weak. The existing `resources` section works well because resource type is a natural, stable boundary — every metric has a clear resource affinity, and users reason about observability this way. Subcategories within a resource type do not have this property.

Consider the full set of optional metrics for Pod: `k8s.pod.cpu_request_utilization`, `k8s.pod.cpu_limit_utilization`, `k8s.pod.cpu.node.utilization`, `k8s.pod.memory_request_utilization`, `k8s.pod.memory_limit_utilization`, `k8s.pod.memory.node.utilization`, `k8s.pod.uptime`, `k8s.pod.volume.usage`, and `k8s.pod.status_reason`. Only the first six form a coherent "utilization" group. The remaining three are unrelated to each other:

- `k8s.pod.uptime` is a single-metric group ("uptime" also exists for container and Node, so at least it is consistent across resources)
- `k8s.pod.volume.usage` measures ephemeral volume bytes — it has no sibling metrics and no natural group name
- `k8s.pod.status_reason` is a status/diagnostics metric from a different receiver (k8s_cluster) — grouping it with the kubeletstats metrics under a shared name is forced

This leads to a taxonomy where one group is clean ("utilization" with 6 metrics) and the rest are either single-metric groups or arbitrary buckets. The group names become a layer of indirection that users must learn, that documentation must maintain, and that can become stale if upstream reorganizes or adds metrics that cross group boundaries.

Additional specific issues:
- Single-metric groups are awkward (for example, a "volumeUsage" group containing one metric)
- The set of valid group names varies per resource type, adding validation complexity
- If upstream metrics are renamed or moved to a different subcategory, the group abstraction breaks

### Option C: Allowed List of Metric Names Per Resource

Add an `additionalMetrics` list field that accepts specific upstream metric names:

```yaml
resources:
  pod:
    enabled: true
    additionalMetrics:
      - k8s.pod.cpu_request_utilization
      - k8s.pod.cpu_limit_utilization
      - k8s.pod.memory_request_utilization
      - k8s.pod.memory_limit_utilization
```

**Pros:**
- Maximum user control — users enable exactly the metrics they need, nothing more
- No intermediate abstraction layer that can become inconsistent with upstream
- Metric names are the same names users see in their backend dashboards and alerting rules — no translation needed
- Every optional metric is addressable, regardless of whether it fits a logical group
- No taxonomy problem: adding a new optional metric upstream only requires adding it to the validation allow-list

**Cons:**
- Users must know the exact upstream metric names
- Direct coupling to upstream metric names — if a metric is renamed, the API value becomes invalid

## Decision

We choose **Option C: allowed list of metric names** (`additionalMetrics`).

Categorizing metrics by resource type is a strong and future-proof abstraction — the existing `resources` section already handles this well. Introducing a second layer of grouping within each resource type is a weak abstraction: some metrics cluster naturally (utilization), but many do not, and subcategory boundaries are likely to shift as receivers evolve.

The allowed list trades abstraction for transparency. Users specify exactly which metrics they want, using the same names they already use in dashboards and alerting rules. The coupling to upstream names is real, but that coupling already exists at every other layer of the user's observability stack.

### Handling Upstream Breaking Changes

None of the three options protect against breaking changes in the upstream receivers. Both receivers are at beta stability, and the optional metrics themselves are at development stability. If a metric is renamed or removed upstream:

1. We must update the validation allow-list to reflect the new names.
2. We must notify users about the change and provide migration guidance.
3. We must roll forward — we cannot keep an old version of the OpenTelemetry Collector indefinitely.

To detect these changes early, we add consistency tests that verify the allow-list matches the metrics defined in the upstream receiver metadata. These tests fail when we bump the collector version and a metric has been renamed or removed, giving us a clear signal to update the allow-list and communicate the change.

## Proposed API

### MetricPipeline Example

```yaml
apiVersion: telemetry.kyma-project.io/v1beta1
kind: MetricPipeline
metadata:
  name: production
spec:
  input:
    runtime:
      enabled: true
      resources:
        pod:
          enabled: true
          additionalMetrics:
            - k8s.pod.cpu_request_utilization
            - k8s.pod.cpu_limit_utilization
            - k8s.pod.memory_request_utilization
            - k8s.pod.memory_limit_utilization
        container:
          enabled: true
          additionalMetrics:
            - k8s.container.cpu_request_utilization
            - k8s.container.cpu_limit_utilization
            - k8s.container.memory_request_utilization
            - k8s.container.memory_limit_utilization
  output:
    otlp:
      endpoint:
        value: "https://backend.example.com:4317"
```

### Type Changes

```go
type MetricPipelineRuntimeInputResource struct {
    // Enabled specifies that the runtime metrics for the resource type
    // are collected. The default is true.
    // +kubebuilder:validation:Optional
    Enabled *bool `json:"enabled,omitempty"`

    // AdditionalMetrics specifies optional upstream metric names to
    // collect in addition to the default curated set. Each entry must
    // be a valid metric name for this resource type. Unknown names are
    // rejected by the validating webhook.
    // +kubebuilder:validation:Optional
    AdditionalMetrics []string `json:"additionalMetrics,omitempty"`
}
```

### Validation

A validating webhook rejects unknown metric names at admission time. The operator maintains a hardcoded allow-list per resource type, derived from the upstream receiver metadata:

| Resource    | Allowed values for `additionalMetrics`                                                                  | Source          |
|-------------|--------------------------------------------------------------------------------------------------------|-----------------|
| Pod         | `k8s.pod.cpu_request_utilization`, `k8s.pod.cpu_limit_utilization`, `k8s.pod.cpu.node.utilization`, `k8s.pod.memory_request_utilization`, `k8s.pod.memory_limit_utilization`, `k8s.pod.memory.node.utilization`, `k8s.pod.uptime`, `k8s.pod.volume.usage`, `k8s.pod.status_reason` | kubeletstats, k8s_cluster |
| Container   | `k8s.container.cpu_request_utilization`, `k8s.container.cpu_limit_utilization`, `k8s.container.cpu.node.utilization`, `k8s.container.memory_request_utilization`, `k8s.container.memory_limit_utilization`, `k8s.container.memory.node.utilization`, `container.uptime`, `k8s.container.status.state`, `k8s.container.status.reason` | kubeletstats, k8s_cluster |
| Node        | `k8s.node.uptime`, `k8s.node.condition`                                                               | kubeletstats, k8s_cluster |
| Volume      | —                                                                                                      | —               |
| Deployment  | —                                                                                                      | —               |
| DaemonSet   | —                                                                                                      | —               |
| StatefulSet | —                                                                                                      | —               |
| Job         | —                                                                                                      | —               |

Resource types that have no optional metrics upstream (volume, Deployment, DaemonSet, StatefulSet, Job) reject any value in `additionalMetrics`.

## Consequences

### Positive

- Users can enable exactly the metrics they need without paying for unnecessary backend storage.
- The default behavior does not change — existing MetricPipeline CRs continue to work without modification.
- The API is transparent: metric names in the CR match the names in dashboards and upstream documentation.
- The allow-list is easy to extend when new optional metrics appear upstream.
- Consistency tests catch upstream metric renames or removals early during collector version bumps.

### Negative

- Users must know the exact upstream metric names. The documentation must list all allowed values per resource type.
- If upstream renames a metric, users must update their MetricPipeline CRs. This is a breaking change that requires customer notification and migration guidance.
- The validating webhook's allow-list must be maintained in sync with the upstream receiver metadata.
