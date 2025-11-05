# Filtering and Processing Data

The Telemetry module provides several ways to refine your telemetry data. You can control what data is collected, enrich it with valuable context, modify its structure, and drop irrelevant information before it reaches your backend.

This processing happens at two distinct stages in the data lifecycle:

## Input-Level Filtering

You can select or reject data at the source, before it enters a pipeline. This is useful for controlling data volume from specific namespaces, applications, or Kubernetes resources. This method is efficient as it prevents unwanted data from being processed at all.

The Telemetry module supports the following input-level filtering mechanisms:

- **Pipeline input filtering** (LogPipeline, MetricPipeline): Configure the `input` section of a pipeline to select or reject data before it is processed by the agent. This is the most common method for filtering application logs, runtime metrics, and Prometheus metrics. For details, see [Filter Logs](filter-logs.md), [Filter Metrics](filter-metrics.md), and [Filter Traces](filter-traces.md).
- **Source-level filtering** (Istio `Telemetry` CRD): For Istio-generated data (access logs and traces), configure the Istio `Telemetry` resource itself to control which workloads generate data and at what volume (sampling rate). See [Filter Logs](filter-logs.md) and [Filter Traces](filter-traces.md) for details.

## Pipeline Processing with OTTL

After data is collected into a pipeline, you can perform custom transformations and filtering using the OpenTelemetry Transformation Language (OTTL). This gives you fine-grained control to modify attributes, redact sensitive information, or drop data based on complex conditions. See [Custom Pipeline Processing with OTTL](custom-pipeline-processing-ottl.md).

## Automatic Processing

In addition to these user-configurable methods, the Telemetry module also performs several automatic processing steps to ensure your data is structured and useful:

- **Data transformation**: Application logs from containers are automatically transformed into structured OpenTelemetry (OTLP) log records. See [Transformation to OTLP Logs](transformation-to-otlp-logs.md).
- **Data enrichment**: All pipelines automatically enrich telemetry data with Kubernetes resource attributes, such as Pod name, namespace, and labels, so you can easily identify the source of telemetry data in your backend. See [Automatic Data Enrichment](automatic-data-enrichment.md).
