# Filtering and Processing Data

The Telemetry module provides several ways to refine your telemetry data. You can control what data is collected, enrich it with valuable context, modify its structure, and drop irrelevant information before it reaches your backend.

## Input-Level Filtering

You can select or reject data at the source, before it enters a pipeline. This is useful for controlling data volume from specific namespaces, applications, or Kubernetes resources. This is the most efficient way to drop large volumes of unwanted data at the earliest stage of the pipeline.

The Telemetry module supports the following input-level filtering mechanisms:

- **Filtering within a pipeline**: In your LogPipeline and MetricPipeline, you can configure the input section to select or reject data before it is processed by the agent. This is the most common method for filtering application logs, runtime metrics, and Prometheus metrics.
- **Configuring the data source**: For Istio-generated data (access logs and traces), you configure the Istio Telemetry resource itself. This controls which workloads generate data and at what volume (sampling rate), before it is even sent to a pipeline.

## Advanced Processing with OTTL

Next, you can use the OpenTelemetry Transformation Language (OTTL) for content-based control over your telemetry data. OTTL rules are applied after basic input filtering. You can modify attributes, redact sensitive information, or drop data based on complex conditions.

## Automatic Processing

By default, Telemetry pipelines perform some automatic processing to standardize your data and make it easier to analyze:

- Application logs from containers are automatically transformed into structured OpenTelemetry (OTLP) log records.
- All pipelines automatically enrich telemetry data with Kubernetes resource attributes, such as Pod name, namespace, and labels. With this context information, you can easily identify the source of telemetry data in your backend.