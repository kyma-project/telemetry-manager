---
title: Ops Scrape Diagnostic Metrics
status: Proposed
date: 2026-06-12
---

# Ops Scrape Diagnostic Metrics

## Context and Problem Statement

The metric agent's Prometheus receivers produce per-target diagnostic metrics for every scrape operation. These metrics report the health and behavior of individual scrape targets:

| Metric | Description |
|--------|-------------|
| `up` | Whether the target is reachable (1 = healthy, 0 = failed) |
| `scrape_duration_seconds` | Time taken to scrape the target |
| `scrape_samples_scraped` | Number of samples the target exposed (pre-relabeling) |
| `scrape_samples_post_metric_relabeling` | Samples remaining after `metric_relabel_configs` |
| `scrape_series_added` | Approximate number of new series added per scrape |
| `scrape_body_size_bytes` | Response body size (-1 = body size limit exceeded, 0 = other failure) |
| `scrape_timeout_seconds` | Configured scrape timeout (static config value) |
| `scrape_sample_limit` | Configured sample limit (static config value) |

`scrape_timeout_seconds` and `scrape_sample_limit` do not provide any value since they are based on static configuration and are excluded from further consideration. 

### The Problem

These metrics are currently dropped before reaching user backends (controlled by `diagnosticMetrics.enabled` in the MetricPipeline spec) and are not exposed for internal monitoring. We have no visibility into whether scrape targets are healthy, hitting limits, or timing out.

See [#2955](https://github.com/kyma-project/telemetry-manager/issues/2955).

### Cardinality

The scrape diagnostic metrics themselves are not a cardinality concern in absolute terms. At 6 metrics × n targets, even 300 pods produce only 1,800 series — manageable for any Prometheus instance.

However, we do not control the amount of workload. It is impossible to predict how many scrape targets exist, and with increasing pod count, the metric agent's Prometheus exporter must hold more series in memory. The series count scales linearly with cluster size, and each series carries multiple labels from `resource_to_telemetry_conversion` (`k8s_pod_name`, `k8s_namespace_name`, `k8s_node_name`, etc.), which inflates per-series memory cost.

### Per-Metric Assessment

The metric agent has three Prometheus scrape jobs: `app-pods`, `app-services`, and `istio`.

| Metric                                   | Decision | Purpose                                                                                           |
|------------------------------------------|----------|---------------------------------------------------------------------------------------------------|
| `up`                                     | Keep     | Health/alerting baseline                                                                          |
| `scrape_samples_scraped`                 | Keep     | Finds the offending target; the metric `sample_limit: 50000` is enforced against (pre-relabeling) |
| `scrape_series_added`                    | Keep     | Churn signal: detects cardinality spikes                                                          |
| `scrape_duration_seconds`                | Keep     | Catches targets slow to serialize large metrics pages (approaching timeout)                       |
| `scrape_body_size_bytes`                 | Keep     | Sentinel: alert on -1 (body size limit hit) or 0 (failure)                                        |
| `scrape_samples_post_metric_relabeling`  | Drop     | Only meaningful where `metric_relabel_configs` exist.                                             |

`scrape_samples_post_metric_relabeling` is equal to `scrape_samples_scraped` in `prometheus` input scrape jobs. It is only useful in `istio` input scrape jobs for identifying how much metric series are discarded after relabeling. It does not provide much information as to why a scrape failed, therefore we can safely discard this metric.

Using a combination of these metrics, ops can identify the root cause of scrape failures:

| Failure mode                    | `up`   |  `scrape_body_size_bytes` | `scrape_duration_seconds ` | `scrape_samples_scraped`  |
|---------------------------------|--------|---------------------------|----------------------------|---------------------------|
| Target unreachable              | 0      | 0                         | low                        | 0                         |
| Scrape timeout                  | 0      | 0                         | ≈ scrape_interval          | 0                         |
| Body size limit exceeded (20MB) | 0      | -1                        | varies                     | varies                    |
| Sample limit exceeded (50000)   | 0      | 0                         | varies                     | ≥ 50000                   |
| Healthy                         | 1      | > 0                       | low                        | > 0                       |

`scrape_body_size_bytes` is only interesting when its value is 0 or -1, because these values indicate that a scrape failed due to exceeding the body size limit or some other error. We can filter the metrics so that we only expose unhealthy metrics for `scrape_body_size_bytes`.

### Aggregation Considerations

You cannot aggregate at scrape time — Prometheus `metric_relabel_configs` only drops, keeps, or rewrites labels. Aggregation happens downstream in the OTel Collector using the `metricstransform` or `transform` processor.

Per-metric aggregation guidance:

| Metric | Function | Rationale |
|--------|----------|-----------|
| `scrape_samples_scraped` | `max` | Hunting the outlier (the target with the most series); `sum` smears it |
| `scrape_series_added` | `max` | Churn concentrates in one target |
| `scrape_duration_seconds` | `max` | The outlier approaching timeout is what you keep this metric to catch; `p95`/`p99` suppress the exact outlier because it's a gauge |
| `up` | `count` / `min` | `count(up == 0)` for failure count; `min` as an "any target down?" boolean |
| `scrape_body_size_bytes` | Never aggregate | Aggregation destroys the -1 sentinel meaning |
| `scrape_samples_post_metric_relabeling` | `max` | Same as `scrape_samples_scraped` |

Always aggregate using a function — never a bare `labeldrop` that leaves live replicas with identical label identities, causing series collisions.

**Advantages of aggregation:**
- Bounded cardinality regardless of cluster size (series count = number of jobs × aggregation groups, not number of pods)
- Safe for self-monitor ingestion — no risk of OOM from workload growth
- Reduces metric agent memory since the Prometheus exporter holds fewer series
- `max` aggregation surfaces the worst target per group, which is the actionable signal

**Disadvantages of aggregation:**
- Lose per-pod attribution — you know "some target in the istio job has 48,000 samples" but not *which* pod
- For churn debugging, per-pod detail is required to find the specific proxy — requires switching to unaggregated mode
- Adds investigation latency: must flip override, wait for next scrape cycle, then observe
- Requires `metricstransform` processor in the collector image (dependency)
- Aggregation dimension choice is non-obvious (by job? by namespace? by node?) — each choice loses different information
- `max` across all targets hides multi-target degradation (5 targets at 40,000 samples looks the same as 1 target at 40,000)

## Considered Options

### Option A: Full Per-Target Exposure (No Aggregation)

Add a `metrics/ops-scrape-metrics` pipeline that:
1. Receives metrics from the enrichment routing connector (prometheus + istio sources)
2. Filters by metric name to keep only diagnostic metrics
3. Exports all datapoints via a Prometheus exporter on port 9090

### Option B: Combined Value-Based Filtering and Aggregation

Apply different strategies per metric based on their nature:
- **Value-based filtering** for metrics where only unhealthy values are interesting:
  - `up`: drop when value == 1 (healthy targets), keep only failures
  - `scrape_body_size_bytes`: drop when value > 0 (normal body size), keep only sentinel values (-1, 0)
- **Aggregation** for numeric diagnostics where the worst case is the actionable signal:
  - `max(scrape_duration_seconds)` by job
  - `max(scrape_samples_scraped)` by job
  - `max(scrape_series_added)` by job

Per-target detail can be restored on demand using the `telemetry-overrides` ConfigMap to bypass aggregation and expose unaggregated metrics for debugging. But this requires a restart of the Metric Agent.

### Comparison

| Criteria                  | Option A                                                                              | Option B                                                                                                                |
|---------------------------|---------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------|
| Per-target visibility     | Full — can identify exactly which pod/proxy is problematic                            | Partial — `up == 0` and `scrape_body_size_bytes` retain per-target identity; aggregated metrics lose it                 |
| Target counting           | `count(up)` gives total targets; `count(up == 0)` gives failure count                 | Cannot count total targets (healthy ones are dropped)                                                                   |
| Preventive measures       | Can detect when scrape samples approach the sample limit for a specific target        | Cannot — only surfaces the `max` per job, loses per-target trending                                                     |
| Cardinality               | Scales linearly with cluster size (5 metrics × n targets) — unpredictable memory cost | Bounded — filtered metrics scale with number of *unhealthy* targets only; aggregated metrics produce one series per job |
| Self-monitor risk         | Risk of OOM in large clusters with many scrape targets                                | Safe — cardinality is bounded regardless of cluster size                                                                |
| Multi-target degradation  | Visible — each target reports independently                                           | Hidden — `max` across all targets hides the case where multiple targets degrade simultaneously                          |
| Implementation complexity | Simple — no aggregation logic, no extra processors                                    | Requires `metricstransform` or `transform` processor for aggregation                                                    |
| Failure-mode distinction  | Can distinguish "target not found" from "target unhealthy" from "target slow" per pod | Can distinguish for filtered metrics (`up`, `scrape_body_size_bytes`); aggregated metrics lose this                     |
| Switching to full detail  | Already full detail                                                                   | Requires override config change and pod restart                                                                         |

## Decision

*To be decided after cardinality testing with 100+ pods.*

## Implementation Notes

### Architecture

The ops scrape metrics pipeline sits after the enrichment pipeline and before the output pipelines:

```
routing/enrichment ──┬──> metrics/output-{user-pipeline}  (existing)
                     └──> metrics/ops-scrape-metrics       (new)
                            ├─ filter/ops-keep-scrape-metrics
                            ├─ filter/ops-drop-healthy-scrape-metrics (Option B only)
                            └─ prometheus/ops-scrape-metrics (:9090)
```

The pipeline receives from the `routing/enrichment` connector. Scrape diagnostic metrics always pass through enrichment because they never match the skip-enrichment criteria (which applies only to runtime resource metrics like `node*`, `deployment*`).

### Prometheus Exporter Configuration

The Prometheus exporter on port 9090 uses:
- `resource_to_telemetry_conversion.enabled: true` — promotes OTel resource attributes (like `k8s.pod.name`) to Prometheus metric labels for target identification
- `metric_expiration: 5m` — automatically removes stale series when a target disappears, preventing unbounded growth from pod churn

### Network Policy and Istio

Port 9090 is included in the metric agent's network policy ingress rules and excluded from Istio sidecar interception through `traffic.sidecar.istio.io/excludeInboundPorts`.

### Self-Monitor Integration

The self-monitor uses `role: endpoints` service discovery, which scrapes every pod behind a Service individually. If we add port 9090 to the existing `telemetry-metric-agent-metrics` Service (or create a dedicated Service), all DaemonSet pods are scraped — giving the full union of scrape diagnostics across all nodes.

### Future: Conditional Per-Target Detail via Overrides

If aggregation is chosen (Option B), the override mechanism uses the existing `telemetry-overrides` ConfigMap:
1. Add a `metricstransform` processor to the ops pipeline that aggregates by default
2. Gate per-target mode behind the overrides ConfigMap
3. Use hot reload (ConfigMap watch) — no pod restart required
4. Scope the override to also restore trimmed Istio peer dimensions
5. Template it as a per-cluster boolean flip ahead of time
