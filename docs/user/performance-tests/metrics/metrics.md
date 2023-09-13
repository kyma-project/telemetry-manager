# Hardened Metric Agent setup

The aim of this exercise is to harden the metric agent such that it can satisfy the metric load of most of the use cases.

## Setup

For the test environment following things were considered:
- Provisioned a GCP cluster with kubernetes
- Deploy Telemetry operator using `make deploy-dev`
- Deploy prometheus for visualizing the metrics
- Istio deployment is needed due to Prometheus

Config map of the metrics agent

Configuration changes needed for metrics agent

## Testcases

### Assumptions

We tweak metrics and series value .... Run it for 1 hour to have stabilized output as we dont want to scale at once (which would cause OOM)

We identified following test cases:
1. Multiple pods all running on a single node and export metrics (to find how many workloads supported)
2. Workload generating huge amount of metrics (To understand how scraping works when the workload exposes several MB of metrics)
3. Have multiple workloads across different nodes (To understand prometheus SDS behaviour with multiple services)
4. Verify istio metrics
5. Test with huge metric payload where we don't scale gradually more like a spike

### Multiple pods all running on a single node and export metrics


### Workload generating huge amount of metrics


### Have multiple workloads across different nodes

-

## Summary




