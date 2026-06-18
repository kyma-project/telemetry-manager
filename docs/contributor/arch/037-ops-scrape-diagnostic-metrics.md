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
| `scrape_samples_scraped` | Number of samples the target exposed |
| `scrape_samples_post_metric_relabeling` | Samples remaining after `metric_relabel_configs` |
| `scrape_series_added` | Approximate number of new series added |
| `scrape_body_size_bytes` | Response body size (-1 = body size limit exceeded, 0 = other failure) |
| `scrape_timeout_seconds` | Configured scrape timeout (static config value) |
| `scrape_sample_limit` | Configured sample limit (static config value) |

The last two (`scrape_timeout_seconds`, `scrape_sample_limit`) report static configuration values that are identical across all targets within a scrape job. They provide no per-target insight and are excluded from further consideration.

### The Problem

These metrics are currently dropped before reaching user backends (controlled by `diagnosticMetrics.enabled` in the MetricPipeline spec) and are not exposed for internal monitoring. We have no visibility into whether scrape targets are healthy, hitting limits, or timing out.

See [#2955](https://github.com/kyma-project/telemetry-manager/issues/2955).

### Cardinality Challenge

Scrape diagnostic metrics are inherently **per-target**: each metric produces one time series per scrape target. The cardinality formula is:

```
number_of_metric_names × number_of_scrape_targets
```

For a cluster with 300 scraped pods and 6 diagnostic metrics, that produces 1,800 series per cluster. Because the metric agent runs as a DaemonSet, the total series across the cluster scales with node count × targets-per-node.

This stands in contrast to all existing metrics in the self-monitor, which have **bounded cardinality**:
- `otelcol_*` metrics scale with the number of pipelines and components, not workload count
- The `otelcol_k8s_pod_association` metric was intentionally redesigned to avoid per-pod cardinality (see [open-telemetry/opentelemetry-collector-contrib#48094](https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/48094))

### Cardinality Reduction Options Evaluated

| Approach | Feasible | Reason |
|----------|----------|--------|
| Aggregate across targets (sum/avg) | No | Loses the per-target detail that makes these metrics useful for debugging |
| Drop labels (instance) | No | Causes series collisions in Prometheus; doesn't reduce cardinality |
| Recording rules for aggregation | Partial | Still requires ingesting full per-target series first |
| Expose only unhealthy datapoints (value-based filter) | Yes | Bounds cardinality to number of *problematic* targets |
| Keep all `up` values, filter others by value | Yes | Preserves target count while limiting cardinality for other metrics |

Aggregation fundamentally defeats the purpose of these metrics: their value lies in identifying *which specific target* has a problem.

### Diagnosis Matrix

Using a combination of diagnostic metrics, ops can identify the root cause of scrape failures:

| Failure mode | `up` | `scrape_body_size_bytes` | `scrape_duration_seconds` | `scrape_samples_scraped` |
|---|---|---|---|---|
| Target unreachable | 0 | 0 | low | 0 |
| Scrape timeout | 0 | 0 | ≈ scrape_interval | 0 |
| Body size limit exceeded | 0 | -1 | varies | varies |
| Sample limit exceeded | 0 | 0 | varies | ≥ 50000 |
| Healthy | 1 | > 0 | low | > 0 |

Additionally, `scrape_samples_post_metric_relabeling` is useful for detecting if the Istio scrape job's `metric_relabel_configs` are accidentally dropping all metrics (value drops to 0 while `scrape_samples_scraped` remains normal).

## Considered Options

### Option A: Expose All Diagnostic Metrics on a Dedicated Port (No Filtering by Value)

Add a `metrics/ops-scrape-metrics` pipeline that:
1. Receives metrics from the enrichment routing connector (prometheus + istio sources)
2. Filters by metric name to keep only diagnostic metrics
3. Exports all datapoints via a Prometheus exporter on port 9090

**Pros:**
- Full visibility into all targets (healthy and unhealthy)
- `count(up)` gives total target count; `count(up == 0)` gives failure count
- Simple implementation with no value-based logic

**Cons:**
- Full per-target cardinality: 6 × number_of_targets series
- Cannot be ingested into the self-monitor without risking overload in large clusters
- Intended for external scraping only (ops must deploy their own Prometheus)

### Option B: Expose Only Unhealthy Datapoints (Value-Based Filtering)

Same as Option A, but add a second filter processor that drops datapoints with healthy values:
- Drop `up == 1` (healthy targets)
- Drop `scrape_body_size_bytes > 0` (normal body size)

**Pros:**
- Cardinality bounded to number of *problematic* targets (near-zero in a healthy cluster)
- Safe to ingest into the self-monitor
- Reduces noise: only actionable signals are visible

**Cons:**
- Lose total target count (`count(up)` only counts unhealthy targets)
- "Empty port 9090" when everything is healthy could be confusing
- Requires the OTel filter processor to support `datapoint.value_int`/`datapoint.value_double` in `metric_conditions`

### Option C: Hybrid — Keep All `up`, Filter Others by Value

Keep the `up` metric unfiltered (all targets, both healthy and unhealthy). Apply value-based filtering only to the other diagnostic metrics:
- Keep all `up` datapoints (full cardinality for this one metric)
- Drop `scrape_body_size_bytes > 0`
- Keep `scrape_duration_seconds`, `scrape_samples_scraped`, `scrape_samples_post_metric_relabeling` only for unhealthy targets (where `up == 0`)

**Pros:**
- Total target count available from `count(up)`
- Failure count from `count(up == 0)`
- Low cardinality for the detailed diagnostic metrics
- Actionable when combined with the diagnosis matrix

**Cons:**
- `up` still has per-target cardinality (1 series per target)
- More complex filter logic
- Detailed diagnostics are only available for failed targets; slow-but-successful scrapes are invisible

## Decision

*To be decided after further discussion.*

## Implementation Notes

### Architecture

The ops scrape metrics pipeline sits after the enrichment pipeline and before the output pipelines:

```
routing/enrichment ──┬──> metrics/output-{user-pipeline}  (existing)
                     └──> metrics/ops-scrape-metrics       (new)
                            ├─ filter/ops-keep-scrape-metrics
                            ├─ filter/ops-drop-healthy-scrape-metrics (Option B/C)
                            └─ prometheus/ops-scrape-metrics (:9090)
```

The pipeline receives from the `routing/enrichment` connector. Scrape diagnostic metrics always pass through enrichment (they never match the skip-enrichment criteria, which applies only to runtime resource metrics like `node*`, `deployment*`, etc.).

### Prometheus Exporter Configuration

The Prometheus exporter on port 9090 uses:
- `resource_to_telemetry_conversion.enabled: true` — promotes OTel resource attributes (like `k8s.pod.name`) to Prometheus metric labels for target identification
- `metric_expiration: 5m` — automatically removes stale series when a target disappears, preventing unbounded growth from pod churn

### Network Policy and Istio

Port 9090 is included in the metric agent's network policy ingress rules and excluded from Istio sidecar interception through `traffic.sidecar.istio.io/excludeInboundPorts`.

### Self-Monitor Integration (Future)

If value-based filtering proves sufficient to bound cardinality (Option B or C), the ops scrape metrics can be integrated into the self-monitor's scrape configuration. This requires adding a Service for port 9090 and a corresponding scrape job. Without filtering, ingesting these metrics into the self-monitor risks cardinality issues in large clusters.
