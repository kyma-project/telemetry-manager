# 4. Do not make Prometheus part of Istio service mesh

Date: 30.01.2024

## Status

Accepted

## Context

As outlined in [ADR 001: Trace/Metric Pipeline status based on OTel Collector metrics](./001-otel-collector-metric-based-pipeline-status.md), our objective is to utilize a managed Prometheus instance to reflect specific telemetry flow issues (such as backpressure, data loss, backend unavailability) in the status of a telemetry pipeline custom resource (CR).
To address the integration of Prometheus querying into the reconciliation loop, a [Proof of Concept was executed](./003-integrate-prometheus-with-telemetry-manager-using-alerting.md).

For the querying to be successful, the Telemetry Manager should be able to reach Prometheus. Because the Telemetry Manager is not in the Istio service mesh, we must decide whether we should have the Prometheus server as part of he Istio service mesh or not.

## Decision
The Telemetry manager which queries metrics from Prometheus is not part of the service mesh.

## Problem
When Prometheus is not part of Istio service mesh, it would cause metrics data to be transported unencrypted, thus the metrics could be counterfeited. This would mean that we get wrong information about the possible issue with observability components. The side effects could be following:
 - Customer gets wrongly notified because of false positives.
 - Telemetry operator gets wrong decision about the scaling: like scaling down when scaling up is needed causing data loss

## Argument 
- The network policy which enables telemetry manager to accept data from a desired IP address in kubernetes cluster reduces the attack vector. It also increases the attack complexity as the attacker would need to have access to the underlying node to perform the attack.
- Making the prometheus part of the Istio service mesh would add a strong dependency to istio service mesh. In case of any issue in the Istio service mesh there will be no monitoring data from Prometheus available thus Status information and Scaling decisions would not be reliable.

## Summary
Based on above arguments of complex attack vector and unreachable monitoring information in case of problems with Istio service mesh, its decided prometheus not to be part of the Istio service mesh.

