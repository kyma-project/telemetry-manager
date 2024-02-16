# 7. Telemetry Flow Healthiness Status API

Date: 2024-15-02

## Status

Proposed

## Context

As a follow-up of [ADR 003: Integrate Prometheus With Telemetry Manager Using Alerting](003-integrate-prometheus-with-telemetry-manager-using-alerting.md),
let's actually define the Telemetry Healthiness Status API for Trace and Metric Pipelines.

In general, we want to reflect interesting events the telemetry flow as Status conditions. Such interesting events are depicted in the following diagram:

![OTel Collector Data Flow](../assets/otel-collector-data-flow.svg "OTel Collector Data Flow")

### Gateway Backpressure / Throttling

* Triggered by Memory Limiter or potential receiver issues (increase in `otelcol_receiver_refused_metric_points` and `otelcol_processor_refused_metric_points`).
* gRPC status code Unavailable returned to the client.
* Scaling may help if the backend is healthy, but could worsen things if not.

### Filter and Transform Processors Refusing Data

* Most failures are checked at startup (OTTL syntax), causing collector crashes. Errors may occur with specific functions like ParseJSON().
* When `error_mode == propagate`, the processor discards data.
* Data is refused, not dropped, and a signal goes back to the receiver for client error notification.

### Exporter Queueing

* Queue fills up when consumers are slower than producers.
* If the exporter queue is full, data is dropped (otelcol_exporter_enqueue_failed_metric_points goes up).
* Watch for high queue size: otelcol_exporter_queue_size / otelcol_exporter_queue_capacity > SOME_THRESHOLD.

### Exporter Retries

* For non-retryable errors, data is dropped ().
* Retryable errors trigger retries until success or retry limit, then data is dropped.
* `otelcol_exporter_send_failed_metric_points` is increased in both scenarios if data is dropped.
* If data is sent successfully, `otelcol_exporter_sent_metric_points` is increased.

## Decision

type: FlowHealth
reasons: FullDataLoss (exporter failed or overflow) | PartialDataLoss | HighBufferUtilization | Throttling

## Consequences

