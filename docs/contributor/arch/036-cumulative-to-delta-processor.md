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

The OpenTelemetry Collector provides the [cumulativetodelta](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/cumulativetodeltaprocessor) processor for this purpose. [ADR 024](024-dynatrace-cummulative-to-delta-support.md) decided to expose temporality configuration under `spec.output.otlp` in the MetricPipeline API. This ADR refines the implementation by documenting the processor configuration and pipeline placement decisions for the metric agent and OTLP gateway.

See [#3440](https://github.com/kyma-project/telemetry-manager/issues/3440) for more information.

### Background

A **cumulative metric** is a running total that only grows (or resets to zero). Each measurement represents the sum of everything that happened since some starting point. If you check it at 10:00 and see 1,000, then at 10:05 you see 1,250, you know 250 events happened in those five minutes, but you have to subtract to figure that out.

A **delta metric** represents the change during a specific time interval. Instead of "total since the beginning," each value answers "what happened during this window?" The delta metric would simply report 250 for that 10:00-10:05 window.

The processor only converts metrics that have a temporality: Sums (counters) and Histograms. Gauges pass through unchanged.

In OpenTelemetry, a **Sum** has two key properties:
- **Temporality:** cumulative or delta
- **Monotonicity:** monotonic (only increases) or non-monotonic (can go up or down)

A Prometheus **Counter** corresponds to an OTel monotonic cumulative Sum.

## Considered Options

The pipeline contains both operator-controlled processors (filters and transforms applied by the platform) and user-controlled processors (transforms and filters defined in the user's `MetricPipeline` custom resource). The key decision is where to place `cumulativetodelta` relative to user-controlled processors.

### Option A: Before User Transforms

Place `cumulativetodelta` before user-controlled transforms and filters. User OTTL statements operate on delta metrics.

**Pros:**
- Guarantees delta correctness regardless of user OTTL — no risk of user transforms breaking delta calculation
- Series identity is stable by the time `cumulativetodelta` processes metrics

**Cons:**
- Holds per-series state for metrics the user will later filter out, increasing memory consumption
- Users cannot write transform statements and conditions based on cumulative metric values
- Contradicts conventional placement in other OTel deployments (for example, Dynatrace's documented Kubernetes monitoring configuration)

### Option B: After User Transforms

Place `cumulativetodelta` after user-controlled transforms and filters. User OTTL statements operate on cumulative metrics.

**Pros:**
- Keeps the per-series state map smaller (only tracks series that survive user filtering)
- Matches conventional placement in other OTel deployments
- Users can write transform conditions based on cumulative metric values

**Cons:**
- User OTTL that modifies identity attributes (metric name or attribute set) based on data point values can break delta calculation
- Requires documenting safe transform patterns for users

## Decision

### Placement in the Pipeline

We choose **Option B: after user transforms**. The memory savings and alignment with established OTel patterns outweigh the risk of user OTTL interference, which is mitigated through documentation of safe transform patterns.

```yaml
metrics/output:
  processors:
    - filter/dynatrace-filter-by-namespace-otlp-input    # operator-controlled
    - transform/drop-kyma-attributes                      # operator-controlled
    - transform/metricpipeline-user-defined-dynatrace    # user-controlled
    - filter/metricpipeline-user-defined-dynatrace       # user-controlled
    - cumulativetodelta                                   # placed after user processors
    - batch
```

This placement is not applied to the shared enrichment pipeline because it would unnecessarily convert metrics for all backends.

#### Series Identity and Transform Order

`cumulativetodelta` identifies a metric series by the combination of its name and the full set of attributes (see [processor documentation](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/cumulativetodeltaprocessor)). Any change to the attribute set produces a new series, causing a one-interval data gap.

Because user transforms run before `cumulativetodelta`, they must not modify identity attributes based on data point values or timestamps. Conditions must reference stable per-series attributes only:

```yaml
# Safe: condition on stable attribute
- set(attributes["team"], "platform") where attributes["k8s.namespace.name"] == "kyma-system"

# Unsafe: condition on data point value - separates metric series
- set(attributes["load"], "high") where value > 10000
```

### Configuration

Two configuration parameters require decisions: `max_staleness` and `initial_value`.

#### `max_staleness`

This parameter controls how long the processor retains per-series state after the series was last seen. Setting a value that is too high risks higher memory consumption because the processor holds stale metrics for a longer period. Setting a value that is too low risks losing track of cumulativeness, because the processor cannot calculate delta based on previous data anymore.

The processor maintains one state entry per unique metric series. Memory consumption scales linearly with the number of active series.

The following table shows the impact per input type for the metric agent:

| Input                              | Series stability                                                        | Memory concern                                                                      | Recommendation                                                  |
|------------------------------------|-------------------------------------------------------------------------|-------------------------------------------------------------------------------------|-----------------------------------------------------------------|
| Prometheus (annotated workloads)   | Stable – long-lived pods, known scrape interval                         | Controllable – set based on scrape interval                                         | 2-3x scrape interval                                            |
| Istio (sidecar metrics)            | Unstable – high cardinality, pod churn creates new series constantly    | High – state map grows with every unique source/dest/pod/response_code combination  | Hard to tune – short evicts too early, long causes memory bloat |
| Runtime (kubeletstats, k8scluster) | Very stable – one series per node/pod, changes only on lifecycle events | Low – bounded by cluster size                                                       | Can be generous                                                 |

The following table shows the impact per input type for the OTLP gateway:

| Input                          | Series stability                                       | Memory concern                               | Recommendation               |
|--------------------------------|--------------------------------------------------------|----------------------------------------------|------------------------------|
| OTLP receiver (push from apps) | Unknown – no control over what sends or how often      | Unbounded – any app can send any cardinality | Impossible to tune precisely |
| Kyma stats receiver            | Unaffected, sends Gauge metrics                        | None                                         | Unaffected                   |

Because `cumulativetodelta` has one `max_staleness` per component (not per input), use the most generous value that is acceptable for memory:

- **Metric agent:** `max_staleness: 4 * max collection/scrape_interval` – accommodates jitters between scrapes and 3 missed scrapes.
- **OTLP gateway:** `max_staleness: 1h` (default) – no scrape interval to base it on, accommodates infrequent OTLP pushers.

All metric agent inputs default to a 30-second collection/scrape interval. The interval is configurable per-input through the Telemetry CR. The reconciler dynamically computes `max_staleness` as `4 * max(configured intervals)` when building the OTel Collector configuration, so it adjusts automatically when a user changes the collection interval.

#### `initial_value`

The [`initial_value`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/cumulativetodeltaprocessor#configuration) setting controls what happens with the first observed data point of a series (which has no previous value to subtract from). Options are `drop`, `auto`, and `keep`.

**Decision: Use `auto` for both metric agent and OTLP gateway.**

`auto` uses the metric's `StartTimestamp` to decide whether to keep or drop the first point. This works well because the metric agent has mixed inputs:
- Prometheus receiver sets `StartTimestamp=0`, so `auto` drops the first point (same as `drop`)
- kubeletstats and k8scluster receivers set proper StartTimestamps → `auto` can keep the first point for pods that started after the collector

For the OTLP gateway, OTel SDKs set proper StartTimestamps, so `auto` preserves first data points from new pods that start after the collector is already running.

### `metricstarttimeprocessor` Is Not Needed

The [`metricstarttimeprocessor`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/metricstarttimeprocessor) (with `true_reset_point` strategy) rebases cumulative values to start from 0 and sets StartTimestamp. Experimental verification shows that it has **no effect** on `cumulativetodelta` output – both produce identical delta values because the constant offset disappears in subtraction, and `cumulativetodelta` overwrites StartTimestamp with its own value regardless.

**Decision:** Omit `metricstarttimeprocessor` from both pipelines. It adds CPU and memory overhead with no benefit when converting to delta. Test data is available in `otel-perf-test/results/`.

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
  max_staleness: 2m  # 4 × 30s; unit suffix (s/m/h) is required, otherwise interpreted as nanoseconds
  initial_value: auto
```

**OTLP gateway:**

```yaml
cumulativetodelta:
  max_staleness: 1h
  initial_value: auto
```

### Performance Testing

To quantify the resource overhead and throughput impact of the `cumulativetodelta` processor, two benchmark scenarios compare a low-cardinality OTLP push test (simulating the gateway) and a high-cardinality Istio scrape test (simulating the metric agent). In both scenarios, two identical collectors run side by side – Collector A (baseline, no `cumulativetodelta`) and Collector B (with `cumulativetodelta`, `max_staleness: 20m`, `initial_value: auto`). Neither collector uses `metricstarttimeprocessor`.

**Test environment:** local k3d cluster using `otel/opentelemetry-collector-contrib:0.150.0`.

> **Note:** The performance tests used `max_staleness: 20m` (a deliberately generous value to avoid premature eviction during testing). The final production configuration uses `2m` based on the `4 × max(scrape_intervals)` formula.

The following metrics are collected:
- **CPU**: `rate(otelcol_process_cpu_seconds_total[1m])` – 1-minute CPU rate sampled at each interval
- **Memory (RSS)**: `otelcol_process_memory_rss_bytes` – instantaneous gauge of Resident Set Size. RSS is used instead of heap because it represents the total physical memory the process occupies, which is what the OOM killer evaluates and what determines whether a pod exceeds its memory limit.
- **Throughput**: `rate(otelcol_receiver_accepted_metric_points_total[1m])` and `rate(otelcol_exporter_sent_metric_points_total[1m])`
- **Peak values**: Maximum observed value across all samples in the measurement window

#### Test 1: telemetrygen (OTLP Push, Low Cardinality)

**Scenario:** Simulates the OTLP gateway receiving cumulative Sum metrics from application SDKs.

**Load:**
- telemetrygen pushes cumulative Sum metrics via OTLP gRPC
- ~1 unique time series per generator (low cardinality)
- Three load levels: 40 pts/s (low), 100 pts/s (medium), 200 pts/s (high)
- Each phase runs for 180 seconds, preceded by a 60-second warm-up
- Three runs per phase, results averaged

**Collector configuration:**
- Baseline (A): `otlp → memory_limiter → batch → otlp/sink`
- CumToDelta (B): `otlp → memory_limiter → cumulativetodelta → batch → otlp/sink`

**Results:**

| Phase | Metric | Baseline (A) | CumToDelta (B) | Difference |
|-------|--------|--------------|----------------|------------|
| LOW (40 pts/s) | Recv Throughput | 40.0 pts/s | 40.0 pts/s | +0.0% |
| | Export Sent | 38.3 pts/s | 39.0 pts/s | +2.0% |
| | CPU | 2.2m | 2.2m | -2.5% |
| | Memory (RSS) | 181.4 MiB | 182.8 MiB | +0.8% |
| MEDIUM (100 pts/s) | Recv Throughput | 99.8 pts/s | 99.8 pts/s | -0.0% |
| | Export Sent | 96.8 pts/s | 99.0 pts/s | +2.4% |
| | CPU | 3.1m | 3.2m | +4.3% |
| | Memory (RSS) | 186.5 MiB | 189.5 MiB | +1.6% |
| HIGH (200 pts/s) | Recv Throughput | 199.8 pts/s | 200.0 pts/s | +0.1% |
| | Export Sent | 198.3 pts/s | 198.9 pts/s | +0.3% |
| | CPU | 4.4m | 4.6m | +4.8% |
| | Memory (RSS) | 191.0 MiB | 192.8 MiB | +1.0% |

No failed exports or refused points in either collector across all phases.

**Conclusion:** With low cardinality, the `cumulativetodelta` processor adds negligible overhead – less than 5% CPU increase and less than 2% memory increase at all load levels. Throughput is unaffected.

#### Test 2: Istio High-Cardinality with Pod Churn

**Scenario:** Simulates the metric agent scraping Istio sidecar metrics under realistic pod churn conditions.

**Load:**
- 15 nginx pods with Istio sidecars (each exposing ~300 envoy/Istio metrics)
- Pod churn: CronJob deletes all pods every 5 minutes (new pods get new IPs → new label combinations → cardinality growth)
- Traffic generation: 60 parallel curl clients every minute creating cross-pod HTTP traffic (generating `istio_requests_total` and related metrics with unique source/destination pairs)
- Scrape interval: 15 seconds
- Measurement duration: 15 minutes (15 samples at 60-second intervals)

**Collector configuration:**
- Baseline (A): `prometheus → memory_limiter → batch → otlp/sink`
- CumToDelta (B): `prometheus → memory_limiter → cumulativetodelta → batch → otlp/sink`

Both collectors scrape the same set of pods using Kubernetes service discovery targeting `istio-proxy` containers on port 15090.

**Results (averages over 15 minutes):**

| Metric | Baseline (A) | CumToDelta (B) | Difference |
|--------|--------------|----------------|------------|
| Recv Throughput | 365.8 pts/s | 361.8 pts/s | -1.1% |
| Export Sent | 363.7 pts/s | 252.3 pts/s | -30.6% |
| CPU (avg) | 20m | 20m | -1.3% |
| Memory RSS (avg) | 250.2 MiB | 267.7 MiB | +7.0% |
| Peak CPU | 39m | 40m | +0.9% |
| Peak Memory RSS | 259.6 MiB | 289.7 MiB | +11.6% |

**Key observations:**

1. **30.6% fewer exported points** – The traffic-generator pods live approximately 24 seconds with a 15-second scrape interval, resulting in only one to two scrapes per pod before it terminates. With `initial_value: auto`, the first scrape is dropped (no previous value to compute delta from). For pods with only one scrape, 100% of their data is lost. For pods with two scrapes, 50% is lost. The long-lived churn-server pods (5-minute lifetime, ~20 scrapes) lose only ~5% to first-point drops.
2. **+7% average memory / +11.6% peak memory** – The per-series state map grows with every unique metric series. Pod churn creates new series continuously (new pod IP → new label combination), and these persist in the state map until `max_staleness` evicts them. Over 15 minutes, Collector B's memory grew from 259 MiB to 303 MiB (versus A: 256 MiB to 272 MiB).
3. **CPU is essentially unchanged** – Delta computation (subtraction) is cheap per-point. The CPU overhead is negligible even with high cardinality.

**Note on the 30.6% drop:** This reflects a worst-case workload pattern with extremely short-lived pods. In production clusters without aggressive pod churn, the drop rate is much lower. The long-lived churn-server pods demonstrate the normal case: ~5% first-point loss, which is negligible.

## Consequences

### Positive Consequences

- Dynatrace receives delta metrics and no longer rejects cumulative sums.
- The pipeline configuration is consistent with Dynatrace's documented OTel deployment patterns, reducing the maintenance burden of a non-standard setup.

### Negative Consequences

- One data point is lost per metric series per pod lifecycle (first-point drop) — negligible for long-lived pods, more significant for short-lived pods.
- The metric agent's memory consumption increases proportionally to the number of active metric series (per-series state map).
- High pod churn (for example, Istio with frequent restarts) causes temporary memory growth until stale series are evicted after `max_staleness`.
- User-defined transforms in `MetricPipeline` operate on cumulative metrics (because `cumulativetodelta` runs after them in the Dynatrace pipeline). Transforms that modify metric identity attributes based on data point values or timestamps can silently break delta calculation. This constraint must be documented in user-facing `MetricPipeline` documentation, with examples of safe and unsafe transform patterns.
