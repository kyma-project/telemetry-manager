---
title: Scrape Diagnostic Metrics
status: Proposed
date: 2026-06-22
related:
  - https://github.com/kyma-project/telemetry-manager/issues/2955
---

# Prometheus Scrape Diagnostic Metrics

## Context and Problem Statement

The metric agent's Prometheus receivers produce per-target diagnostic metrics for every scrape operation (`up`, `scrape_duration_seconds`, `scrape_samples_scraped`, `scrape_samples_post_metric_relabeling`, `scrape_series_added`, `scrape_body_size_bytes`, `scrape_timeout_seconds`, `scrape_sample_limit`). These metrics report the health and behavior of individual scrape targets.

### Problem

These metrics are currently dropped before reaching user backends (controlled by `diagnosticMetrics.enabled` in the `MetricPipeline` spec) and are not exposed for internal monitoring. There is no visibility into whether scrape targets are healthy, hitting limits, or timing out.

See [#2955](https://github.com/kyma-project/telemetry-manager/issues/2955).

### Cardinality

The scrape diagnostic metrics themselves are not a cardinality concern in absolute terms. At 8 metrics × n targets, even 300 pods produce only 2,400 series, which is manageable for any Prometheus instance.

However, the number of scrape targets is unpredictable, and the series count scales linearly with the number of targets. Additionally, each series carries multiple labels from `resource_to_telemetry_conversion` (`k8s_pod_name`, `k8s_namespace_name`, `k8s_node_name`, etc.) and inflates per-series memory cost.

### Per-Metric Assessment

The metric agent has four Prometheus scrape jobs: `app-pods`, `app-services`, `istio`, and `app-services-secure`.

| Metric                                   | Decision | Purpose                                                                                           |
|------------------------------------------|----------|---------------------------------------------------------------------------------------------------|
| `up`                                     | Keep     | Health/alerting baseline (1 = healthy, 0 = failed)                                                |
| `scrape_samples_scraped`                 | Keep     | Finds the offending target; the `sample_limit: 50000` is enforced against this value (pre-relabeling) |
| `scrape_series_added`                    | Keep     | Churn signal: detects cardinality spikes                                                          |
| `scrape_duration_seconds`                | Keep     | Catches targets slow to serialize large metrics pages (approaching timeout)                       |
| `scrape_body_size_bytes`                 | Keep     | Sentinel: -1 = body size limit exceeded, 0 = other failure, > 0 = healthy response size           |
| `scrape_samples_post_metric_relabeling`  | Keep     | Samples remaining after `metric_relabel_configs`; only meaningful in `istio` scrape jobs           |
| `scrape_timeout_seconds`                 | Keep     | Static config value; kept for consistency with user-facing diagnostic metrics                     |
| `scrape_sample_limit`                    | Keep     | Static config value; kept for consistency with user-facing diagnostic metrics                     |

`scrape_timeout_seconds` and `scrape_sample_limit` are static configuration values identical for all targets within a scrape job. They do not provide diagnostic signal but are kept for consistency with the user-facing diagnostic metrics (exposed via `diagnosticMetrics.enabled`) and to avoid maintaining a separate metric name list.

`scrape_samples_post_metric_relabeling` is equal to `scrape_samples_scraped` in `prometheus` input scrape jobs. It is only useful in `istio` and input scrape jobs for identifying how many metric series are discarded after relabeling.

A combination of these metrics identifies the root cause of scrape failures:

| Failure mode                    | `up`   | `scrape_body_size_bytes` | `scrape_duration_seconds` | `scrape_samples_scraped`  |
|---------------------------------|--------|--------------------------|---------------------------|---------------------------|
| Target unreachable              | 0      | 0                        | low                       | 0                         |
| Scrape timeout                  | 0      | 0                        | ≈ scrape_interval         | 0                         |
| Body size limit exceeded (20MB) | 0      | -1                       | varies                    | varies                    |
| Sample limit exceeded (50000)   | 0      | 0                        | varies                    | ≥ 50000                   |
| Healthy                         | 1      | > 0                      | low                       | > 0                       |

## Considered Options

### Option A: Always-On Exposure (No Aggregation)

Expose all 8 scrape diagnostic metrics per target to the internal monitoring system at all times.

This approach has the following trade-offs:

- Full per-target attribution for debugging
- No additional pipeline complexity
- Unbounded cardinality risk, see [Cardinality](#cardinality)
- Memory pressure on the internal monitoring system

### Option B: Always-On Exposure with Aggregation

Expose aggregated scrape metrics (for example, max per scrape job) to bound cardinality. Because Prometheus cannot perform aggregation, this happens downstream in the OpenTelemetry (OTel) Collector using the `metricstransform` or `transform` processor.

Per-metric aggregation guidance:

| Metric                                  | Function          | Rationale                                                             |
|-----------------------------------------|-------------------|-----------------------------------------------------------------------|
| `scrape_samples_scraped`                | `max`             | Finding the outlier (the target with the most series)                 |
| `scrape_series_added`                   | `max`             | Churn concentrates in one target                                      |
| `scrape_duration_seconds`               | `max`             | The outlier approaching timeout is what you keep this metric to catch |
| `up`                                    | `count` / `min`   | `count(up == 0)` for failure count                                    |
| `scrape_body_size_bytes`                | Never aggregate   | Aggregation destroys the -1 sentinel meaning                          |
| `scrape_samples_post_metric_relabeling` | `max`             | Same as `scrape_samples_scraped`                                      |

This approach has the following trade-offs:

- Bounded cardinality regardless of number of targets
- Safe for self-monitor ingestion
- Loses per-pod attribution. The result tells you "some target in the istio job has 48,000 samples" but not which pod.
- For churn debugging, per-pod detail is required to find the specific proxy
- Requires additional dependency in the collector image (`metricstransform` processor)

### Option C: On-Demand Exposure via Override (Chosen)

Expose scrape metrics only when explicitly enabled through the `telemetry-overrides` ConfigMap. On its own, on-demand exposure lets scrape failures go unnoticed until users report missing metrics. To close this gap, the self-monitor continuously evaluates scrape health using aggregated alerting rules and surfaces issues as conditions on the `MetricPipeline` CR. The self-monitor provides cluster-level detection without per-target cardinality cost, and when it fires, operators enable on-demand metrics for per-target investigation.

This approach has the following trade-offs:

- Zero cost by default
- Full per-target detail when needed for investigation
- No additional processor dependencies
- Self-monitor closes the detection gap
- Requires manual intervention to enable detailed metrics during incidents
- Metrics are not available for historical analysis

## Decision

Expose scrape diagnostic metrics on-demand using the `telemetry-overrides` ConfigMap. The self-monitor continuously monitors scrape health and alerts on issues. This approach avoids storing unnecessary metric series in the internal monitoring system while still identifying clusters with problems. When the self-monitor detects an issue within a cluster, operators can enable on-demand scrape metrics for detailed investigation. A separate ADR covers the self-monitor implementation.

**Rationale:**
- Always-on exposure carries unbounded cardinality risk in clusters with many scrape targets.
- Aggregation loses per-target attribution, which is essential for debugging scrape failures.
- On-demand exposure avoids both problems: zero cardinality cost by default, full per-target detail when needed.

**Behavior:**
- By default, port 9090 on the metric agent serves an empty Prometheus metrics response (no series).
- The internal monitoring system continuously scrapes this endpoint without triggering target-down alerts.
- When `metrics.prometheusScrapeMetricsEnabled: true` is set in the `telemetry-overrides` ConfigMap, the metric agent exposes all Prometheus scrape metrics to port 9090.
- The internal monitoring system picks up the metrics on the next scrape cycle, and they become available in the dashboard.

## Consequences

### Positive

- No cardinality cost or memory overhead during normal operation.
- Operators retain full per-target diagnostic detail for incident investigation.
- Consistent metric name set with the user-facing `diagnosticMetrics.enabled` feature reduces maintenance burden.
- Port 9090 is always listening, so the internal monitoring system does not generate false target-down alerts.

### Negative

- Scrape health metrics are not available for historical trending (only available from the moment of enablement).
- Incident response requires a manual step: updating the `telemetry-overrides` ConfigMap before diagnostic data becomes available.
- The metric agent's memory increases when on-demand metrics are enabled. Testing with 100 scrape targets showed ~4–5 MB additional memory usage. Operators must disable the override after investigation to reclaim memory.

## Implementation Notes

### Architecture

The ops scrape metrics pipeline sits after the enrichment pipeline and before the output pipelines:

![Architecture diagram with ops scrape metrics pipeline](../assets/prometheus-scrape-diagnostic-metrics.png)

The Prometheus exporter on port 9090 is always present in the OTel Collector configuration. When the override is disabled, no data flows to it (the pipeline is not connected to the enrichment routing connector). When the override is enabled, a routing table entry with `action: copy` matches the 8 diagnostic metric names and copies them to the ops pipeline. The existing routing entries remain unchanged.

The `metrics/ops-scrape-metrics` pipeline always exists in the service configuration so that the Prometheus exporter starts and listens on port 9090. The gating mechanism is the `routing/enrichment` connector's routing table:

**Override disabled (default):** The routing connector does not route to the ops pipeline.

```yaml
connectors:
    routing/enrichment:
        table:
            - context: metric
              statement: "route() where ..."
              pipelines:
                  - metrics/output-{user-pipeline}
```

**Override enabled:** The routing connector prepends a copy entry for diagnostic metrics.

```yaml
connectors:
    routing/enrichment:
        table:
            - context: metric
              statement: "route() where metric.name == \"up\" or ..."
              pipelines:
                  - metrics/ops-scrape-metrics
              action: copy
            - context: metric
              statement: "route() where ..."
              pipelines:
                  - metrics/output-{user-pipeline}
```

The `action: copy` entry matches only diagnostic metrics and copies them to the ops pipeline without affecting the subsequent routing entries. The ops pipeline has no filter processor: the routing condition itself acts as the filter.

### Prometheus Exporter Configuration

The Prometheus exporter on port 9090 uses:
- `resource_to_telemetry_conversion.enabled: true`: Promotes OTel resource attributes such as `k8s.pod.name` to Prometheus metric labels for target identification.
- `metric_expiration: 5m`: Automatically removes stale series when a target disappears, preventing unbounded growth from pod churn.

### Network Policy and Istio

Port 9090 is included in the metric agent's network policy ingress rules and excluded from Istio sidecar interception through `traffic.sidecar.istio.io/excludeInboundPorts`.

### Override Configuration

The `telemetry-overrides` ConfigMap in the `kyma-system` namespace controls the feature:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: telemetry-overrides
  namespace: kyma-system
data:
  overrides: |
    metrics:
      prometheusScrapeMetricsEnabled: true
```

Changing this value triggers a reconciliation that rebuilds the metric agent configuration with or without the ops pipeline data flow.
