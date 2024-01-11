# 3. Integrate Prometheus with Telemetry Operator using Alerts API

Date: 2024-01-11

## Status

Accepted

## Context

As outlined in [ADR 001: Trace/Metric Pipeline status based on OTel Collector metrics](./001-otel-collector-metric-based-pipeline-status.md), our objective is to utilize a managed Prometheus instance to reflect specific telemetry flow issues (e.g., backpressure, data loss, backend unavailability) in the status of a telemetry pipeline Custom Resource (CR).
We have previously determined that both Prometheus and its configuration will be managed within the Telemetry Operator's code, aligning with our approach for managing Fluent Bit and OTel Collector.

To address the integration of Prometheus querying into the reconciliation loop, a Proof of Concept was executed. The results from testing the queries also indicate that invoking Prometheus APIs will not significantly impact the overall reconciliation time.
