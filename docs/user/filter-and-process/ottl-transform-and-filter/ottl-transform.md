# Transforming with OTTL

Use transformations to modify telemetry data as it flows through a pipeline. You can add, update, or delete attributes, change metric types, or modify span details to enrich data or conform to a specific schema.

You configure transformations in the `transform` section of a pipeline's spec.

Each transformation rule consists of:

- **statements**: A list of OTTL functions to execute
- **conditions** (optional): A list of OTTL conditions. If you provide conditions, the statements only run if at least one of the conditions is true

## Example: General Resource Attribute Enrichment

This example adds a `deployment.environment.name` attribute with the value `production` to all metrics in the pipeline. Since there are no conditions, the rule applies to all data.

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

## Example: Conditional Resource Attribute Enrichment

This example sets the status code of a trace span to `1` (Error) if its pod name matches `my-pod-name.*` and its `http.path` attribute is `/health`.

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