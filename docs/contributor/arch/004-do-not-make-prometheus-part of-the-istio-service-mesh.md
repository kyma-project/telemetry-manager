# 4. Do not make Prometheus part of Istio service mesh

Date: 30.01.2024

## Status

Accepted

## Context

As outlined in [ADR 001: Trace/Metric Pipeline status based on OTel Collector metrics](./001-otel-collector-metric-based-pipeline-status.md), our objective is to utilize a managed Prometheus instance to reflect specific telemetry flow issues (such as backpressure, data loss, backend unavailability) in the status of a telemetry pipeline custom resource (CR).
To address the integration of Prometheus querying into the reconciliation loop, a [Proof of Concept was executed](./003-integrate-prometheus-with-telemetry-manager-using-alerting.md).

For the querying to be successful, the telemetry operator should be able to reach prometheus. Since operator is not in Istio service mesh we should make conscious decision if we should have Prometheus server part of Istio service mesh or not.

## Decision
The Telemetry operator which queries metrics from Prometheus is not part of the service mesh. Making Prometheus part of the Istio service mesh would require having additional resources such as `permissive` Istio peer authentication policy. This would nullify the advantages brought in by the Istio service mesh.
Currently, the ports for scraping metrics is not part of the service mesh. So based on the arguments, the Prometheus Server should be not be part of the Istio service mesh.

## Consequences
The metrics data would be transported in plain text, thus there is a possibility of it being intercepted and/or counterfeited. This is a security risk that needs to be accepted.
