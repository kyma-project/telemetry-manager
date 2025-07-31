# 21. Decouple MetricPipeline Agent from Gateway

Date: 2025-07-10

## Status

Accepted

## Context

The current Telemetry module architecture defines a metric pipeline that deploys a gateway and an agent. The gateway receives metrics data from applications and metric agents, enriches it, and dispatches it to the backend.
The agent should be decoupled from the gateway and run as a standalone component (see [Switch from Gateways to a Central Agent](019-switch-from-gateways-to-a-central-agent.md)).

In the current setup, the gateway enriches and filters data on behalf of the metric agents. Most of the enrichments are common across all inputs, such as Kubernetes metadata enrichment, but some enrichments are input-specific, such as Istio or runtime enrichments.
The current architecture requires complex cross-input filtering in multi-pipeline scenarios, which is not efficient and leads to configuration complexity.

## Proposal

The decoupling of the agent from the gateway can be done based on receiver type or input type.

- **Receiver type**: This approach requires a separate enrichment and output stage for each receiver type. This leads to unnecessary duplication of components because they share the same data path, and thus increases configuration complexity.
- **Input type**: This approach defines a single enrichment pipeline for all inputs, while supporting dedicated input and output pipelines for the input types (OTLP, Prometheus, Istio, or Runtime).
  All these pipelines are connected with `routing` connectors. The enrichment pipeline shared across all inputs and outputs, allowing for maximum reuse of components.
 

![enrichment](./../assets/metric-enrichment.png)

The new agent configuration consists of three pipelines:

1. **Input**: Defines input-specific pipeline configurations, including input-specific receivers, processors, and an input-specific router to connect to the next pipeline. For each defined input type (Prometheus, Istio, or Runtime), there is a single input pipeline, which is shared across all defined MetricPipelines.
2. **Enrichment**: Defines enrichment-specific pipeline configurations, including enrichment-specific processors and a router to connect to the next pipeline. The enrichment pipeline is shared across all inputs.
3. **Output**: Defines output for a specific MetricPipeline configuration, including processors and exporters (such as namespace filtering).

![config](./../assets/metric-agent-pipelines.png)

See a [sample configuration for the new agent](./../assets/sample-metric-agent-config.yaml).

The metric gateway, like the agent, consists of three pipelines::
1. **Input**: Defines OTLP-input-specific pipeline configuration, and performs input-specific enrichment.
2. **Enrichment**: Defines enrichment-specific pipeline configurations, including enrichment-specific processors and a router to connect to the next pipeline. The enrichment pipeline is shared across all the inputs.
3. **Output**: Defines output for a specific MetricPipeline configuration, including processors and exporters (such as namespace filtering).

![gaetway](./../assets/metric-gateway-pipelines.png)

 - The gateway will be simplified to support only the OTLP receiver and OTLP exporter. It will handle enrichment and filtering exclusively for OTLP input. 
 - An exception is the `kymastats` receiver, which remains on the gateway for now because of the following reasons: 
   - Unlike other receivers on the agents, it does not collect data from node-specific resources.
   - Currently, the gateway is deployed by default, and the `kymastats` receiver is enabled by default. Moving it to the agent means that the agent also must be deployed by default, which increases resource footprint.

See a [sample configuration for the new gateway](./../assets/sample-metric-gateway-config.yaml).

## Conclusion

1. The new MetricPipeline configuration architecture is split into three pipelines: input, enrichment, and output.
2. The input pipeline is defined for each input type, allowing for input-specific processing. The input pipeline configurations are grouped by input type rather than by receiver type, because all receivers in the same input type share the same data path. Creating a separate input pipeline for each receiver type would lead to unnecessary duplication of components and configuration complexity.
3. The enrichment pipeline is shared across all inputs and outputs, allowing for maximum reuse of components and resource efficiency. Otherwise, especially in multi-pipeline scenarios, the `k8sattribute` processor would need its own resource cache per data pipeline, which can lead to high memory usage.
4. The output stage is specific for each MetricPipeline, supporting backend-specific enrichment, filtering, and data export.
5. In multi-pipeline scenarios, each receiver appears only once per input type and processes a combined data stream. Filtering is performed in the output pipeline to ensure only relevant data is exported.
6. In multi-pipeline scenarios, cross-MetricPipeline filtering is no longer needed, because each output pipeline processes only its own data.
