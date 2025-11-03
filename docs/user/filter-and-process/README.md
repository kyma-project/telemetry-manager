# Filtering and Processing Data

When you configure the inputs for your logs, traces, and metrics, you can choose from which specific Kubernetes resources you want to include or exclude data. Your data is automatically transformed and enriched so you can analyze it in your OTLP backend.

## Filtering Mechanisms

The Telemetry module supports the following mechanisms to filter the data, which apply at different stages of the collection process:

- Filtering within a pipeline's `input` section (LogPipeline, MetricPipeline): You can configure the `input` section of a pipeline to select or reject data before it is processed by the agent. This is the most common method for filtering application logs, runtime metrics, and Prometheus metrics.
- Filtering within a pipeline's `filter` section (LogPipeline, MetricPipeline, TracePipeline): You can add filter processors to a pipeline's `filter` section to further refine the data after it has been collected but before it is sent to the backend. This allows for more complex filtering logic based on attributes of the telemetry data.
- Configuring the data source (Istio `Telemetry` CRD): For Istio-generated data (access logs and traces), you configure the Istio `Telemetry` resource itself. This controls which workloads generate data and at what volume (sampling rate), before it is even sent to a pipeline.

## Processing Data

Application logs from containers are automatically transformed into structured OpenTelemetry (OTLP) log records.

All pipelines automatically enrich telemetry data with Kubernetes resource attributes, such as Pod name, namespace, and labels. With this context information, you can easily identify the source of telemetry data in your backend.

If additional custom transformations are desired (such as conditionally adding or modifying attributes), you can use the OTTL Custom Processing feature to define custom processing logic within a pipeline's `transform` section.
