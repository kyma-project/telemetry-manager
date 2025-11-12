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
# In your LogPipeline spec
spec:
  input:
    application:
      enabled: true
  output:
    otlp:
      endpoint:
        value: http://traces.example.com:4317
  transform:
  transform:
    # Try to parse the body as custom parser (default spring boot logback)
    # 2025-11-12T14:40:36.828Z  INFO 1 --- [demo] [           main] c.e.restservice.RestServiceApplication   : Started RestServiceApplication in 23.789 seconds (process running for 30.194)
    - statements:
        - merge_maps(log.attributes, ExtractPatterns(log.body,"^(?P<timestamp>\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}\\.\\d+Z)\\s+(?P<level>[A-Z]+)\\s+(?P<pid>\\d+)\\s+---\\s+\\[(?P<mdc>[^\\]]+)\\]\\s+\\[\\s*(?P<thread>[^\\]]+)\\s*\\]\\s+(?P<logger>[^\\s:]+)\\s*:\\s*(?P<msg>.*)$"), "upsert")
        - merge_maps(log.attributes, log.cache, "upsert") where Len(log.cache) > 0
        - set(log.attributes["parsed"], true) where Len(log.cache) > 0

    # Try to enrich core attributes if custom parsing was successful
    - conditions:
        - log.attributes["parsed"] == true
      statements:
        - set(log.body, log.attributes["msg"])
        - set(log.severity_text, ToUpperCase(log.attributes["level"]))
        - delete_matching_keys(log.attributes, "^(level|msg|parsed|)$")
```

## Example: Masking Sensitive Data

You can redact data using typical patterns so that potential sensitive data is masked.

```yaml
# In your TracePipeline spec
spec:
  output:
    otlp:
      endpoint:
        value: http://traces.example.com:4317
  transform:
      - statements:
          - replace_pattern(span.attributes["http.url"], "client_id=[^&]+", "client_id=[REDACTED]")
          - replace_pattern(span.attributes["http.url"], "client_secret=[^&]+", "client_secret=[REDACTED]")
```
