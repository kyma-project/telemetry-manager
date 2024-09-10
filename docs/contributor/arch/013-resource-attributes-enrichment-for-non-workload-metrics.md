# 2. Resource Attributes Enrichment for Non-Workload Metrics

Date: 2024-09-10

## Status

Accepted

## Context

Currently, the [k8sattributes processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/k8sattributesprocessor/README.md) is used in the Metric Gateway to enrich metrics resource attributes with Kubernetes metadata.
This is very useful for workload metrics (like Pod and container metrics). However, for non-workload metrics (like Node metrics), the metrics will be incorrectly associated with Metric Agent Pod, because the Metric Agent Pod is the one that emits these metrics.

For example, in the following diagram, the `k8s.node.cpu.usage`metric is incorrectly associated with the `telemetry-metric-agent-4ncx9` Pod. This creates confusion, because the user might incorrectly think that this metric provides the Node CPU consumed by the `telemetry-metric-agent-4ncx9` Pod. While in reality, it provides the entire Node CPU usage.
In addition, the metrics from the system namespaces (including `kyma-system` namespace) are excluded by default. Thus, this metric is dropped by default even if the user enables the collection of Node metrics.

![Node Metric With k8sattributes Processor](../assets/node-metric-with-k8sattributes-processor.png)

Therefore, we need to ensure that non-workload metrics are not enriched with unwanted resource attributes.

## Decision

There are different possible solutions to solve this problem:

### Option 1: Using Connectors

![Connectors](../assets/connectors.drawio.svg)

We can split our MetricPipeline into 3 sub-pipelines which are connected using [Connectors](https://opentelemetry.io/docs/collector/configuration/#connectors) as shown in the diagram above.
The `Input Pipeline` contains the `otlp` receiver and `memory_limiter` processor.
The `Attributes Enrichment Pipeline` contains the `k8sattributes` and `transform/resolve-service-name` processors.
The `Output Pipeline` contains the rest of the processors and the `otlp` exporter.

The metrics will be routed to the `Attributes Enrichment Pipeline` only if they are workload metrics. This can be done by checking if a resource attribute (like `skip-enrichment`) is set `true`.
Otherwise, the metrics will bypass the `Attributes Enrichment Pipeline` and will be routed directly to the `Output Pipeline`.

To check the effect of the new setup on the performance of the Metric Gateway, [load tests](https://github.com/kyma-project/telemetry-manager/tree/main/docs/contributor/benchmarks#metric-gateway) were executed using version [1.22.0](https://github.com/kyma-project/telemetry-manager/releases/tag/1.22.0) for the telemetry-manager and the OTel collector image `europe-docker.pkg.dev/kyma-project/dev/kyma-otel-collector:PR-121`, which contains the [routingconnector](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/routingconnector) and [forwardconnector](https://github.com/open-telemetry/opentelemetry-collector/tree/main/connector/forwardconnector) components.
For the single Pipeline load tests, the gateway configuration from [here](../assets/single_pipeline_gateway_with_connectors.yaml) was used.
For the multi Pipeline load tests, the gateway configuration from [here](../assets/multi_pipeline_gateway_with_connectors.yaml) was used.

The results of a single execution of the load tests is shown in the table below:
 
<div class="table-wrapper" markdown="block">


|                       Version/Test | Single Pipeline (ci-metrics) |                              |                     |                      |               | Multi Pipeline (ci-metrics-m) |                              |                     |                      |               | Single Pipeline Backpressure (ci-metrics-b) |                              |                     |                      |               | Multi Pipeline Backpressure (ci-metrics-mb) |                              |                     |                      |               |
|-----------------------------------:|:----------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|:-----------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|:-------------------------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|:-------------------------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|
|                                    | Receiver Accepted Metric/sec | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage | Receiver Accepted Metric/sec  | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage |        Receiver Accepted Metric/sec         | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage |        Receiver Accepted Metric/sec         | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage |
| Current Setup (Without Connectors) |             4458             |             4458             |          0          |       143, 162       |   1.5, 1.5    |             3282              |             9845             |          0          |       219, 256       |   1.8, 1.7    |                     824                     |             638              |         251         |       827, 829       |   0.5, 0.5    |                    1809                     |             1812             |         504         |      1784, 1737      |   1.3, 1.3    |
|                    With Connectors |             4459             |             4461             |          0          |       172, 153       |   1.5, 1.5    |             3166              |             9500             |          0          |       242, 227       |   1.7, 1.7    |                     842                     |             631              |         314         |       908, 921       |   0.5, 0.5    |                    1979                     |             1918             |         509         |      1695, 1712      |   1.4, 1.5    |


To check the variation of the load tests results, the load tests were executed 3 times for each setup. The results are shown in the table below:

|                         Version/Test | Single Pipeline (ci-metrics) |                              |                     |                      |               |
|-------------------------------------:|:----------------------------:|:----------------------------:|:-------------------:|:--------------------:|:-------------:|
|                                      | Receiver Accepted Metric/sec | Exporter Exported Metric/sec | Exporter Queue Size | Pod Memory Usage(MB) | Pod CPU Usage | 
| 1.Current Setup (Without Connectors) |             4458             |             4458             |          0          |       143, 162       |   1.5, 1.5    |             
| 2.Current Setup (Without Connectors) |             4476             |             4476             |          0          |       138, 153       |   1.6, 1.5    |
| 3.Current Setup (Without Connectors) |             4395             |             4396             |          0          |       146, 163       |   1.6, 1.5    |
|                   1. With Connectors |             4459             |             4461             |          0          |       172, 153       |   1.5, 1.5    |
|                   2. With Connectors |             4476             |             4479             |          0          |       167, 138       |   1.5, 1.5    |
|                   3. With Connectors |             4471             |             4471             |          0          |       157, 163       |   1.5, 1.5    |

As a conclusion of the load tests results, the performance of the Metric Gateway is not affected by the new setup with Connectors. The results show that the performance of the Metric Gateway is similar for both setups.

</div>

- _Pros_: 
  - Clean solution. The non-workload metrics will never have the unwanted resource attributes set to any value.
  - This is the recommended solution for doing a conditional routing in an OTel collector pipeline.
  - If a user sends their own custom non-workload metrics, they can skip enriching their custom non-workload metrics with unwanted resource attributes by setting a resource attribute (like `skip-enrichment`) to `true`. This will be documented for the users, so that they would be aware of the possibility of skipping the resource attributes enrichment.
- _Cons_:
  - Per MetricPipeline, we will have 3 pipelines in the collector instead of 1. So, we will have a more complex setup of the pipeline service definitions in combination with the new connectors definitions.

### Option 2: Setting Unwanted Resource Attributes With Dummy Values

We can explicitly set the unwanted resource attributes with dummy values for non-workload metrics in the Metric Agent.
Then, we can delete all the resource attributes with dummy values in the Metric Gateway.

- _Pros_:
  - If someone inspects the metrics emitted by the Metric Agent, it will be clear that the resource attributes with the dummy values are not desired.
- _Cons_: 
  - If a user sends their own custom non-workload metrics, there is no option for them to skip the unwanted resource attributes.

### Option 3: Directly Deleting Unwanted Resource Attributes

We can directly delete the unwanted resource attributes in the Metric Gateway after they have been incorrectly enriched by the k8sattributes processor.

- _Pros_:
  - Simplest solution, because we will just need to add a single processor in the existing setup for deleting the unwanted resource attributes.
- _Cons_:
  - If a user sends their own custom non-workload metrics, there is no option for them to skip the unwanted resource attributes.
  - If a user deploys their own OTel Collector and sends metrics to the Metric Gateway, they might be explicitly setting the resource attributes that we are deleting with custom values.

We have decided to adopt option 1. Although the Metric Gateway configuration will become more complex, it is the cleanest and recommended solution for doing a conditional routing in an OTel collector pipeline and the only solution which allows users to skip enriching their custom non-workload metrics with unwanted resource attributes.

## Consequences

This change will ensure that non-workload metrics are not enriched with unwanted resource attributes in the Metric Gateway.
