---
title: CumulativeToDelta Processor for Metric Pipelines
status: Proposed
date: 2026-05-07
---

# CumulativeToDelta Processor for Metric Pipelines

## Context and Problem Statement

Dynatrace rejects metrics with `MONOTONIC_CUMULATIVE_SUM` temporality. The metric agent and OTLP gateway must convert cumulative metrics to delta temporality before exporting to Dynatrace. Without this conversion, the backend returns errors like:

```
the endpoint only accepts the delta aggregation temporality for MONOTONIC_CUMULATIVE_SUM
```

The OpenTelemetry Collector provides the [cumulativetodelta](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/cumulativetodeltaprocessor) processor for this purpose. This ADR documents the configuration decisions for integrating this processor into the metric agent and OTLP gateway pipelines.

See [#3440](https://github.com/kyma-project/telemetry-manager/issues/3440) for more information.

### Background

A **cumulative metric** is a running total that only grows (or resets to zero). Each measurement represents the sum of everything that happened since some starting point. If you check it at 10:00 and see 1,000, then at 10:05 you see 1,250, you know 250 events happened in those five minutes, but you have to subtract to figure that out.

A **delta metric** represents the change during a specific time interval. Instead of "total since the beginning," each value answers "what happened during this window?" The delta metric would simply report 250 for that 10:00-10:05 window.

The processor only converts metrics that have a temporality: Sums (counters) and Histograms. Gauges pass through unchanged.

In OpenTelemetry, a **Sum** has two key properties:
- **Temporality:** cumulative or delta
- **Monotonicity:** monotonic (only increases) or non-monotonic (can go up or down)

A Prometheus **Counter** corresponds to an OTel monotonic cumulative Sum.

## Decision

### Configuration

Two configuration parameters require decisions: `max_staleness` and `initial_value`.

#### `max_staleness`

This parameter controls how long the processor retains per-series state after the series was last seen. Setting a value that is too high risks higher memory consumption because the processor holds stale metrics for a longer period. Setting a value that is too low risks losing track of cumulativeness, because the processor cannot calculate delta based on previous data anymore.

The processor maintains one state entry per unique metric series. Memory consumption scales linearly with the number of active series.

**Impact per input type (metric agent):**

| Input                              | Series stability                                                        | Memory concern                                                                      | Recommendation                                                  |
|------------------------------------|-------------------------------------------------------------------------|-------------------------------------------------------------------------------------|-----------------------------------------------------------------|
| Prometheus (annotated workloads)   | Stable — long-lived pods, known scrape interval                         | Controllable — set based on scrape interval                                         | 2-3x scrape interval                                            |
| Istio (sidecar metrics)            | Unstable — high cardinality, pod churn creates new series constantly    | High — state map grows with every unique source/dest/pod/response_code combination  | Hard to tune — short evicts too early, long causes memory bloat |
| Runtime (kubeletstats, k8scluster) | Very stable — one series per node/pod, changes only on lifecycle events | Low — bounded by cluster size                                                       | Can be generous                                                 |

**Impact per input type (OTLP gateway):**

| Input                          | Series stability                                       | Memory concern                               | Recommendation               |
|--------------------------------|--------------------------------------------------------|----------------------------------------------|------------------------------|
| OTLP receiver (push from apps) | Unknown — no control over what sends or how often      | Unbounded — any app can send any cardinality | Impossible to tune precisely |
| Kyma stats receiver            | Unaffected, sends Gauge metrics                        | None                                         | Unaffected                   |

Since `cumulativetodelta` has one `max_staleness` per component (not per input), use the most generous value that is acceptable for memory:

- **Metric agent:** `max_staleness: 20m` — accommodates Istio pod churn (5-minute restarts) while limiting stale series accumulation
- **OTLP gateway:** `max_staleness: 1h` (default) — no scrape interval to base it on, accommodates infrequent OTLP pushers

As a reference, the Dynatrace OTel Collector Documentation uses [25 hours](https://docs.dynatrace.com/docs/ingest-from/opentelemetry/collector/use-cases/prometheus) as its example configurations.

#### `initial_value`

The processor works by subtracting consecutive readings. If you see `1000` at 10:00 and `1070` at 10:01, the delta is 70. But the very first reading has nothing to subtract from. The `initial_value` setting determines what happens with that first point.

The three options:

- **`drop`**: Always discards the first observed value. Guarantees no double-counting but loses one data point per series.
- **`auto`**: Uses the metric's `StartTimestamp` to decide. If the counter started after the processor (meaning it is a genuinely new series), keeps the first point. Otherwise, drops it. The logic:
  ```
  if StartTimestamp < processor_start_time → drop (counter predates collector)
  if ObservedTimestamp == StartTimestamp   → drop (first observation ever)
  otherwise                               → keep
  ```
- **`keep`**: Always sends the first observed value as the delta. Correct only when the collector's lifecycle is tied to the metric source (for example, sidecar deployments).

**Decision: Use `auto` for both metric agent and OTLP gateway.**

The metric agent has multiple inputs with different StartTimestamp behaviors:
- Prometheus receiver sets `StartTimestamp=0` since `auto` drops first point (same as `drop`)
- kubeletstats and k8scluster receivers set proper StartTimestamps → `auto` can keep the first point for pods that started after the collector

Since `cumulativetodelta` has one `initial_value` per pipeline, `auto` handles both correctly: drops when it cannot trust the timestamp, keeps when it can.

For the OTLP gateway, OTel SDKs set proper StartTimestamps, so `auto` can preserve first data points from new pods that start after the collector is already running.

### `metricstarttimeprocessor` Is Not Needed

The `metricstarttimeprocessor` (with `true_reset_point` strategy) rebases cumulative values to start from 0 and sets StartTimestamp. We verified experimentally that it has **no effect** on `cumulativetodelta` output.

**Test setup:** Two collectors scraping the same static counter pod (incrementing by 10/s, scraped every 10s):

| Pipeline | Config |
|----------|--------|
| Collector C | `prometheus → cumulativetodelta → debug` |
| Collector D | `prometheus → metricstarttime (true_reset_point) → cumulativetodelta → debug` |

**Results:** Both produced identical delta values (100.0 per interval) across 64 data points, with both `initial_value: drop` and `initial_value: auto`.

**Why the deltas are identical:**

```
Raw from Prometheus:        500, 600, 700, 800  (cumulative since pod start)
After metricstarttime:        0, 100, 200, 300  (cumulative since first observation, rebased to 0)
After cumulativetodelta:         100, 100, 100  (delta per scrape interval)
```

Without `metricstarttimeprocessor`, `cumulativetodelta` sees the raw values:
- `600 - 500 = 100`, `700 - 600 = 100`, `800 - 700 = 100`

With `metricstarttimeprocessor`, the values are rebased but deltas are identical:
- `100 - 0 = 100`, `200 - 100 = 100`, `300 - 200 = 100`

The constant offset disappears in subtraction. Additionally:
- `cumulativetodelta` overwrites StartTimestamp with the previous scrape time, making the timestamp set by `metricstarttimeprocessor` irrelevant
- The 1970 epoch StartTimestamp problem does not exist in delta output
- Counter reset detection is value-based (`value < prevValue`), not StartTimestamp-based

**Decision:** Omit `metricstarttimeprocessor` from both pipelines. It adds CPU and memory overhead with no benefit when converting to delta.

### Counter Resets

When a workload pod restarts, its counters reset to 0. The processor detects this by comparing values:

```
Scrape N:   value=700
Scrape N+1: value=0    ← value < previous → reset detected → drop
Scrape N+2: value=10   → delta = 10 - 0 = 10 (normal)
```

One data point is lost per restart per metric series. This is unavoidable regardless of configuration, because the processor cannot compute a meaningful delta across a reset boundary.

### Final Configuration

**Metric agent:**

```yaml
cumulativetodelta:
  max_staleness: 20m
  initial_value: auto
```

**OTLP gateway:**

```yaml
cumulativetodelta:
  max_staleness: 1h
  initial_value: auto
```

## Consequences

- Dynatrace receives delta metrics and no longer rejects cumulative sums
- One data point is lost per metric series per pod lifecycle (first-point drop) — negligible for long-lived pods, more significant for short-lived pods
- The metric agent's memory consumption increases proportionally to the number of active metric series (per-series state map)
- High pod churn (for example, Istio with frequent restarts) causes temporary memory growth until stale series are evicted after `max_staleness`
- `metricstarttimeprocessor` is not included in the pipeline, reducing CPU overhead
- The `max_staleness` configuration can be exposed as a configurable parameter in the MetricPipeline CR for edge cases with extreme pod churn
