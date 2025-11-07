# Transform with OTTL

Use transformations to modify telemetry data as it flows through a pipeline. You can add, update, or delete attributes, change metric types, or modify span details to enrich data or conform to a specific schema.

## Overview

> [!NOTE]
> This feature is based on the underlying OpenTelemetry Transform Processor. For more implementation details, syntax, and examples, see the [OTel Transform Processor Documentation](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/transformprocessor/README.md). However, bear in mind that some limitations and differences exist, as outlined in the [Limitations](./README.md#limitations) section.

You define these rules in the `transform` section of your Telemetry pipeline's `spec`.

Each rule in the `transform` list contains:
- `statements`: One or more OTTL functions that modify the data. These are the actions that you want to perform.
- `conditions` (optional): One or more OTTL conditions that must be met. If you provide conditions, the statements only run on data that matches at least one of the conditions.

If you don't provide any conditions, the statements apply to all telemetry data passing through the pipeline.

> [!TIP] Remember
> Filters run **after** all transformations. Your filter conditions must operate on the final, modified state of your data, not its original state.

## Example: Add a Global Attribute to All Metrics

You can add a `deployment.environment.name` attribute with the value production to all metrics. This is useful for tagging all data from a specific cluster.

```yaml
# In your MetricPipeline spec
spec:
  input:
    prometheus:
      enabled: true
  output:
    otlp:
      endpoint:
        value: http://metrics.example.com:4317
  transform:
    - statements:
        - 'set(resource.attributes["deployment.environment.name"], "production")'
```

## Example: Conditionally Set a Span's Status

You can mark a trace span as an error if it comes from a specific pod and represents a failing health check:

```yaml
# In your TracePipeline spec
spec:
  output:
    otlp:
      endpoint:
        value: http://traces.example.com:4317
  transform:
    - conditions:
        - 'IsMatch(resource.attributes["k8s.pod.name"], "my-pod-name.*")'
      statements:
        - 'set(span.status.code, 1) where span.attributes["http.path"] == "/health"'
```