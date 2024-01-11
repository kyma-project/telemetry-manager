# 3. Integrate Prometheus with Telemetry Operator using Alerts API

Date: 2024-01-11

## Status

Accepted

## Context

As outlined in [ADR 001: Trace/Metric Pipeline status based on OTel Collector metrics](./001-otel-collector-metric-based-pipeline-status.md), our objective is to utilize a managed Prometheus instance to reflect specific telemetry flow issues (e.g., backpressure, data loss, backend unavailability) in the status of a telemetry pipeline Custom Resource (CR).
We have previously determined that both Prometheus and its configuration will be managed within the Telemetry Manager's code, aligning with our approach for managing Fluent Bit and OTel Collector.

To address the integration of Prometheus querying into the reconciliation loop, a Proof of Concept was executed.

## Decision

The results of the query tests affirm that invoking Prometheus APIs won't notably impact the overall reconciliation time. In theory, we could directly query Prometheus within the Reconcile routine. However, this straightforward approach presents a few challenges:

### Timing of Invocation
Our current reconciliation strategy triggers either when a change occurs or every minute. While this is acceptable for periodic status updates, it may not be optimal when considering future plans to leverage Prometheus for autoscaling decisions.

### Flakiness Mitigation
To ensure reliability and avoid false alerts, it's crucial to introduce a delay before signaling a problem. As suggested in [OTel Collector monitoring best practices](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/monitoring.md):

> Use the rate of otelcol_processor_dropped_spans > 0 and otelcol_processor_dropped_metric_points > 0 to detect data loss. Depending on requirements, set up a minimal time window before alerting to avoid notifications for minor losses that fall within acceptable levels of reliability.

If we directly query Prometheus, we would need to independently implement such a mechanism to mitigate flakiness.
