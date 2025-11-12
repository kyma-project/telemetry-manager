# Transform with OTTL

Use transformations to modify telemetry data as it flows through a pipeline. You can modify telemetry fields to enrich data or conform to a specific schema.

## Overview

You define these rules in the `transform` section of your Telemetry pipeline's `spec`.

Each rule in the `transform` list contains:

- `statements`: One or more OTTL functions that modify the data. These are the actions that you want to perform.
- `conditions` (optional): One or more OTTL conditions that must be met. If you provide conditions, the statements only run on data that matches at least one of the conditions.

If you don't provide any conditions, the statements apply to all telemetry data passing through the pipeline.

> [!TIP]
> - The pipeline applies all transformation rules **before** it evaluates any filter rules. Any change you make during the transformation affects the data that your filters see.
> - This feature is based on the [OpenTelemetry Transform Processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/transformprocessor/README.md), with some limitations (see [Limitations](./README.md#limitations)).

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

If you want to identify system-related spans in your observability backend,  mark all spans from workloads in system namespaces by adding a **system** attribute with the value `true`.

```yaml
# In your TracePipeline spec
spec:
  output:
    otlp:
      endpoint:
        value: http://traces.example.com:4317
  transform:
    - conditions:
        - 'IsMatch(resource.attributes["k8s.namespace.name"], ".*-system")'
      statements:
        - 'set(span.attributes["system"], "true")'
```

## Example: Custom Log Parsing

You are using a LogPipeline with the `application` input and your application logs in a custom format to stdout. Then you might want to parse the payload and enrich core attributes.

```yaml
spec:
  input:
    application:
      enabled: true
  output:
    otlp:
      endpoint:
        value: http://traces.example.com:4317
  transform:
    # Try to parse the body as custom parser (python)
    - conditions:
        - log.attributes["parsed"] == nil
      statements:
        - merge_maps(log.attributes, ExtractPatterns(log.body, "File\\s+\"(?P<filepath>[^\"]+)\""), "upsert")
        - merge_maps(log.attributes, log.cache, "upsert") where Len(log.cache) > 0
        - set(log.attributes["parsed"], true) where Len(log.cache) > 0

    # Try to enrich core attributes if custom parsing was successful
    - conditions:
        - log.attributes["parsed"] != nil
      statements:
        - set(log.body, log.attributes["message"]) where log.attributes["message"] != nil
        - set(log.severity_number, SEVERITY_NUMBER_DEBUG) where IsMatch(log.attributes["level"], "(?i)debug")
        - set(log.severity_number, SEVERITY_NUMBER_INFO) where IsMatch(log.attributes["level"], "(?i)info")
        - set(log.severity_number, SEVERITY_NUMBER_WARN) where IsMatch(log.attributes["level"], "(?i)warn")
        - set(log.severity_number, SEVERITY_NUMBER_ERROR) where IsMatch(log.attributes["level"], "(?i)err")
        - set(log.severity_text, ToUpperCase(log.attributes["level"])) where log.severity_number > 0
        - delete_matching_keys(log.attributes, "^(level|message|parsed|)$")
```
