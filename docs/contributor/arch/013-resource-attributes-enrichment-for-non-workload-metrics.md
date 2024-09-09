# 2. Resource Attributes Enrichment for Non-Workload Metrics

Date: 2024-08-30

## Status

Accepted

## Context

Currently, the [k8sattributes processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/k8sattributesprocessor/README.md) is used in the Metric Gateway to enrich metrics resource attributes with Kubernetes metadata.
This is very useful for workload metrics (like Pod and container metrics). However, for non-workload metrics (like Node metrics), the metrics will be incorrectly associated with Metric Agent Pod, because the Metric Agent Pod is the one that emits these metrics.

For example, in the following diagram, the `k8s.node.cpu.usage`metric is incorrectly associated with the `telemetry-metric-agent-4ncx9` Pod. This creates confusion, because the user might incorrectly think that this metric provides the Node CPU consumed by the `telemetry-metric-agent-4ncx9` Pod. While in reality, it provides the entire Node CPU usage.
In addition, the metrics from the system namespaces (including `kyma-system` namespace) are excluded by default. Thus, this metric is dropped by default even if the user enables the collection of Node metrics.

![Node Metric With k8sattributes Processor](../assets/node-metric-with-k8sattributes-processor.png)

Thus, we need to ensure that non-workload metrics are not enriched with unwanted resource attributes.

## Decision

There are different possible solutions to solve this problem:

### Option 1: Using Connectors

![Connectors](../assets/connectors.drawio.svg)

We can split our MetricPipeline into 3 sub-pipelines which are connected using [Connectors](https://opentelemetry.io/docs/collector/configuration/#connectors) as shown in the diagram above.
An `Input Pipeline`, which contains the `otlp` receiver and `memory_limiter` processor.
An `Attributes Enrichment Pipeline`, which contains the `k8sattributes` and `transform/resolve-service-name` processors.
An `Output Pipeline`, which contains the rest of the processors and the `otlp` exporter.

The metrics will be routed to the `Attributes Enrichment Pipeline` only if they are workload metrics.
Otherwise, the metrics will bypass the `Attributes Enrichment Pipeline` and will be routed directly to the `Output Pipeline`.

Load test using the image `europe-docker.pkg.dev/kyma-project/dev/kyma-otel-collector:PR-121`, which contains the `routingconnector` and `forwardconnector` components:
 
<div class="table-wrapper" markdown="block">


|                       Version/Test | Single Pipeline (ci-metrics) |                              |                     |                      |               | Multi Pipeline (ci-metrics-m) |                              |                     |                      |               | Single Pipeline Backpressure (ci-metrics-b) |                              |                     |                      |               | Multi Pipeline Backpressure (ci-metrics-mb) |                              |                     |                      |               |
|-----------------------------------:|:----------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|:-----------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|:-------------------------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|:-------------------------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|
|                                    | Receiver Accepted Metric/sec | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage | Receiver Accepted Metric/sec  | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage |        Receiver Accepted Metric/sec         | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage |        Receiver Accepted Metric/sec         | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage |
| Current Setup (Without Connectors) |             4458             |             4458             |          0          |       143, 162       |   1.5, 1.5    |             3282              |             9845             |          0          |       219, 256       |   1.8, 1.7    |                     824                     |             638              |         251         |       827, 829       |   0.5, 0.5    |                    1809                     |             1812             |         504         |      1784, 1737      |   1.3, 1.3    |
|                    With Connectors |             4149             |             4149             |          0          |       192, 163       |   1.7, 1.6    |             2347              |             7042             |          0          |       247, 303       |   1.6, 1.6    |                     772                     |             682              |         172         |       831, 867       |   0.5, 0.4    |                    1383                     |             1848             |         498         |      1719, 1686      |   1.3, 1.3    |



|                         Version/Test | Single Pipeline (ci-metrics) |                              |                     |                      |               |
|-------------------------------------:|:----------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|
|                                      | Receiver Accepted Metric/sec | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage | 
| 1.Current Setup (Without Connectors) |             4458             |             4458             |          0          |       143, 162       |   1.5, 1.5    |             
| 2.Current Setup (Without Connectors) |             4476             |             4476             |          0          |       138, 153       |   1.6, 1.5    |
| 3.Current Setup (Without Connectors) |             4395             |             4396             |          0          |       146, 163       |   1.6, 1.5    |
|                   1. With Connectors |             4149             |             4149             |          0          |       192, 163       |   1.7, 1.6    |
|                   2. With Connectors |             4155             |             4156             |          0          |       174, 182       |   1.7, 1.6    |
|                   3. With Connectors |             4069             |             4069             |          0          |       191, 181       |   1.6, 1.6    |

</div>

- _Pros_: 
  - Clean solution. The non-workload metrics will never have the unwanted resource attributes set to any value.
  - This is the recommended solution for doing a conditional routing in an OTel collector pipeline.
- _Cons_:
  - Per MetricPipeline, we will have 3 pipelines in the collector instead of 1. So, we will have a more complex setup of the pipeline definitions in combination with the new connectors definitions.

### Option 2: Setting Unwanted Resource Attributes With Dummy Values

We can explicitly set the unwanted resource attributes with dummy values for non-workload metrics in the Metric Agent.
Then, we can delete all the resource attributes with dummy values in the Metric Gateway.

- _Pros_:
  - If someone inspects the metrics emitted by the Metric Agent, it will be clear that the resource attributes with the dummy values are not desired.
- _Cons_: 
  - If a user deploys their own OTel Collector and sends metrics to the Metric Gateway, then the unwanted resource attributes will not be deleted, because they will not have the dummy values.

### Option 3: Directly Deleting Unwanted Resource Attributes

We can directly delete the unwanted resource attributes in the Metric Gateway after they have been incorrectly enriched by the k8sattributes processor.

- _Pros_:
  - Simplest solution, because we will just need to add a single processor in the existing setup for deleting the unwanted resource attributes.
- _Cons_:
  - If a user deploys their own OTel Collector and sends metrics to the Metric Gateway, they might be explicitly setting the resource attributes that we are deleting with custom values.
  - If a user sends their own custom non-workload metrics, there is no option for them to skip the unwanted resource attributes.


We have decided to adopt option 3 because it is the simplest solution and the probability that a customer deploys their own OTel Collector, sends metrics to the Metric Gateway, and sets the resource attributes that we are deleting with custom values is low.

In addition, we will need to delete a different set of resource attributes for each non-workload metrics group.
Therefore, the logic for deleting the unwanted resource attributes will be complex and that is why we will implement a custom processor for this logic instead of using [Transform processors](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/transformprocessor/README.md).

## Consequences

This change will ensure that non-workload metrics are not enriched with unwanted resource attributes in the Metric Gateway.
