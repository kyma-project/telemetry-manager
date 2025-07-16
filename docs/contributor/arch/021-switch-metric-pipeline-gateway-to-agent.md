# 21. Decouple MetricPipeline Agent from Gateway

Date: 2025-07-10

## Status

Proposed

## Context

The current Telemetry module architecture defines a metric pipeline which would deploy a gateway and an agent. The gateway receives metrics data from applications and metric agents, enriches it, and dispatches it to the backend.
The agent should be decoupled from the gateway and run as a standalone component (see [Switch from Gateways to a Central Agent](019-switch-from-gateways-to-a-central-agent.md)).

In the current setup, the gateway enriches and filters data on behalf of the metric agents. Most of the enrichments are common across all inputs, such as Kubernetes metadata enrichment, but some enrichments are input-specific, such as Istio or runtime enrichments.
The current architecture requires complex cross-input filtering in multi-pipeline scenarios, which is not efficient and leads to configuration complexity.

## Proposal

The decoupling of the agent from the gateway can be done based on receiver type or input type.

- **Receiver type**: This approach would require a separate enrichment and output stage for each receiver type. This lead to unnecessary duplication of components as they share the same data path and thus increasing configuration complexity.
- **Input type**: This approach, for all metric pipelines
  - It would define a dedicated input pipeline for the input types OTLP, Prometheus, Istio, or Runtime.
  - It would define a single enrichment pipeline for all inputs.
 
It would define a dedicated output pipeline for the input type. All these pipelines are connected with `routing` connectors. The enrichment pipeline will be shared across all inputs and outputs, allowing for maximum reuse of components.

![enrichment](./../assets/metric-enrichment.png)

The new agent configuration consists of three pipelines:

1. **Input**: Defines input-specific pipeline configurations, including input-specific receivers, processors, and an input-specific router to connect to the next pipeline. The input pipeline exists for each defined input type once (Prometheus, Istio, or Runtime) and shared across all defined MetricPipelines.
2. **Enrichment**: Defines enrichment-specific pipeline configurations, including enrichment-specific processors and a router to connect to the next pipeline. The enrichment pipeline is shared across all the inputs.
3. **Output**: Defines output for a specific MetricPipeline configurations, including processors and exporters (such as namespace filtering).

![config](./../assets/metric-agent-pipelines.png)

See a [sample configuration for the new agent](./../assets/sample-metric-agent-config.yaml).

The metric gateway, like the agent, consists of three pipelines::
1. **Input**: Defines OTLP-input-specific pipeline configuration, and performs enrichment.
2. **Enrichment**: Defines enrichment-specific pipeline configurations, including enrichment-specific processors and a router to connect to the next pipeline. The enrichment pipeline is shared across all the inputs.
3. **Output**: Defines output for a specific MetricPipeline configurations, including processors and exporters (such as namespace filtering).

![gaetway](./../assets/metric-gateway-pipelines.png)

 - The gateway will be simplified to support only the OTLP receiver and OTLP exporter. It will handle enrichment and filtering exclusively for OTLP input. 
 - An exception is the `kymastats` receiver, which will remain on the gateway for now. That's because: 
   - Unlike other receivers on the agents, it does not collect data from node-specific resources.
   - Currently, the gateway is deployed by default, `kymastats` receiver is enabled by default, moving it to the agent would always require an agent deployment, which will increase resource footprint.

See a [sample configuration for the new gateway](./../assets/sample-metric-gateway-config.yaml).

## Conclusion

1. The new MetricPipeline configuration architecture is split into three pipelines: input, enrichment, and output. This approach allows for a clear separation of concerns and simplifies the configuration.
2. The input pipeline is defined for each input type, allowing for input-specific processing. The input pipeline configurations are grouped by input type rather than by receiver type, because all receivers in the same input type share the same data path. Creating a separate input pipeline for each receiver type would lead to unnecessary duplication of components and configuration complexity.
3. The enrichment pipeline is shared across all inputs and outputs, allowing for maximum reuse of components and resource efficiency, otherwise, especially in multi-pipeline scenarios the `k8sattribute` processor will have own resource cache per data pipeline which can result high memory usage.
4. The output stage is MetricPipeline specific, allowing for backend specific enrichment, filtering, and data export.
5. In multi-pipeline scenarios, each receiver appears only once per input type and processes a combined data stream. Filtering is performed in the output pipeline to ensure only relevant data is exported.
6. In multi-pipeline scenarios, cross-MetricPipeline filtering is no longer needed, as each output pipeline processing only own data.