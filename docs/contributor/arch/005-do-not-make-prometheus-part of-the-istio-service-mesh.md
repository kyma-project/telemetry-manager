# 4. Do Not Make Prometheus Part of Istio Service Mesh

Date: 30.01.2024

## Status

Accepted

## Context

As outlined in [ADR 001: Trace/Metric Pipeline status based on OTel Collector metrics](./001-otel-collector-metric-based-pipeline-status.md), our objective is to utilize a managed Prometheus instance to reflect specific telemetry flow issues (such as backpressure, data loss, backend unavailability) in the status of a telemetry pipeline custom resource (CR).
To address the integration of Prometheus querying into the reconciliation loop, a [Proof of Concept was executed](./003-integrate-prometheus-with-telemetry-manager-using-alerting.md).

For the query to be successful, the Telemetry Manager should be able to reach Prometheus. Because the Telemetry Manager is not in the Istio service mesh, we must decide whether we should have the Prometheus server as part of the Istio service mesh or not.

## Decision
The Telemetry Manager, which queries metrics from Prometheus, is not part of the service mesh.

## Problem
When Prometheus is not part of the Istio service mesh, it will cause metrics data to be transported unencrypted. Thus, the metrics could be counterfeited. This would mean we get the wrong information about the possible issue with observability components. The side effects could be the following:
 - Customer gets wrongly notified because of false positives.
 - Telemetry Manager gets the wrong decision about the scaling, like scaling down when scaling up is needed, thus causing data loss

## Argument 
- The network policy that enables Telemetry Manager to accept data from a desired IP address in the Kubernetes cluster reduces the attack vector. It also increases the attack complexity because the attacker would need access to the underlying node to perform the attack.
- Making Prometheus part of the Istio service mesh would add a strong dependency to the Istio service mesh. In case of any issue in the Istio service mesh, there will be no monitoring data from Prometheus available thus, status information and scaling decisions would not be reliable.

## Summary
Based on the arguments of complex attack vector and unreachable monitoring information in case of problems with the Istio service mesh, it's decided that Prometheus will not be part of the Istio service mesh.

