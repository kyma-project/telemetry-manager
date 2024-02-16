# 7. Telemetry Flow Healthiness Status API

Date: 2024-15-02

## Status

Proposed

## Context

As a follow-up of [ADR 003: Integrate Prometheus With Telemetry Manager Using Alerting](003-integrate-prometheus-with-telemetry-manager-using-alerting.md),
let's actually define the Telemetry Healthiness Status API for Trace and Metric Pipelines.

In general, our aim is to highlight significant events in the telemetry flow as status conditions. In the event of issues, it should be apparent to the customer where the problem lies.
Additionally, we should offer runbooks to assist them in resolving these problems. The diagram below visually represents these key events:

![OTel Collector Data Flow](../assets/otel-collector-data-flow.svg "OTel Collector Data Flow")

### Throttling

* Currently, can be only triggered by Memory Limiter (increase in `otelcol_receiver_refused_metric_points` and `otelcol_processor_refused_metric_points`).
There is a community discussion about incorporating a rate-limiting mechanism directly into the OTLP Receiver: https://github.com/open-telemetry/opentelemetry-collector/issues/6725.
* Often triggered by memory pressure from the exporter queue, although during extreme traffic spikes, memory may exceed the soft limit even when the queue is empty.
* gRPC status code Unavailable returned to the client.
* Scaling may help if the backend is healthy, but could worsen things if not.

### Filter and Transform Processors Refusing Data

* Most failures are checked at startup (OTTL syntax), causing collector crashes. Errors may occur with specific functions like ParseJSON().
* When `error_mode == propagate`, the processor refuses data, and a signal goes back to the receiver for client error notification.
* Not relevant for us so far since we neither use `error_mode == propagate` nor use OTTL functions that can cause errors.

### Exporter Queueing

* All batches are enqueued first.
* Queue fills up when consumers are slower than producers. It can be caused by backend issues or a mismatch between the ingestion and export rate (e.g. backend is slow).
* If the exporter queue is full, data is dropped (`otelcol_exporter_enqueue_failed_metric_points` goes up).
* Watch for high queue size: `otelcol_exporter_queue_size / otelcol_exporter_queue_capacity > THRESHOLD`.

### Exporter Retries

* For non-retryable errors, data is dropped. See more about retryable and non-retryable errors.
* Retryable errors trigger retries until success or retry limit, then data is dropped.
* `otelcol_exporter_send_failed_metric_points` is increased in both scenarios if data is dropped.
* If data is sent successfully, `otelcol_exporter_sent_metric_points` is increased.

## Decision

type: FlowHealth
reasons: FullDataLoss (exporter failed or overflow) | PartialDataLoss | HighBufferUtilization | Throttling

## Consequences

