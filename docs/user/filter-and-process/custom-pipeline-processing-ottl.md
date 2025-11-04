# Custom Pipeline Processing with OTTL

> **Note**: This feature is only available with Telemetry Manager v1.52.0 and later.

After your telemetry data has been collected, you can use the custom transform and filter feature with the [OpenTelemetry Transformation Language (OTTL)](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl) to apply advanced, fine-grained processing to your telemetry data, enriching, modifying, and filtering it before it is exported to a backend.

You can define `transform` and `filter` OTTL rules in the `spec` of your LogPipeline, MetricPipeline, and TracePipeline resources. When you define rules, the Telemetry module configures processors in the underlying OpenTelemetry Collector to execute your statements.

## Processing Order

Processing rules are always applied in a specific order. This is critical to understand when designing your rules:

1. **Transformations (transform)**: Data is first modified according to the rules in the `transform` section
2. **Filters (filter)**: The transformed data is then evaluated against the rules in the `filter` section. Any data matching a filter condition is dropped

This sequence ensures that you filter on the final, transformed state of your data. For example, if you rename an attribute in a transform rule, your filter rule must use the new name, not the original one.

## Context Model

All transformation and filtering statements operate on an inferred element context — you do not configure the context separately:

- LogPipeline rules act on individual log records (context: `log`)
- MetricPipeline rules act on individual metric data points (context: `datapoint`)
- TracePipeline rules act on individual spans (context: `span`)

> **Note**: Span events (`spanevent`) are not supported: you cannot transform or filter fields inside individual span events.

Always reference attributes with their full context path. Examples:

- Current element: `log.attributes["level"]`, `datapoint.value`, `span.name`
- Resource / higher scope: `resource.attributes["k8s.namespace.name"]`, `metric.name` (from a datapoint), `scope.name`

Error handling: If a statement fails (for example, referencing a missing attribute), the processor logs the error and continues (ignore mode). One bad record does not stop the pipeline.

These rules ensure predictable behavior without additional context configuration.

## Transforming Telemetry Data

Use transformations to modify telemetry data as it flows through a pipeline. You can add, update, or delete attributes, change metric types, or modify span details to enrich data or conform to a specific schema.

You configure transformations in the `transform` section of a pipeline's spec.

Each transformation rule consists of:

- **statements**: A list of OTTL functions to execute
- **conditions** (optional): A list of OTTL conditions. If you provide conditions, the statements only run if at least one of the conditions is true

### Example: General Resource Attribute Enrichment

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

### Example: Conditional Resource Attribute Enrichment

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

## Filtering Telemetry Data

Use filters to drop unwanted telemetry data from a pipeline. Filtering helps you reduce noise, lower storage and processing costs, and focus on the data that matters most.

You configure filters in the `filter` section of a pipeline's spec. The filter processor drops any data point, span, or log record that matches one of your conditions.

Each filter block consists of a list of conditions. If multiple conditions are provided within a filter block, they are combined with a logical `OR` (any matching condition drops the data).

### Example: Drop Debug Logs

This example drops any log record that has a severity level below warning (e.g., debug and info logs).

```yaml
# In your LogPipeline spec
spec:
  input:
    application:
      enabled: true
  output:
    otlp:
      endpoint:
        value: http://logs.example.com:4317
  filter:
    - conditions:
        - 'severity_number < SEVERITY_NUMBER_WARN'
```

### Filter Envoy metrics to outlier_detection only

This example keeps all regular Istio metrics but filters Envoy metrics (those prefixed with `envoy_`) to only include outlier detection metrics, which are essential for monitoring circuit breaker behavior and host ejections in your service mesh.

```yaml
# In your MetricPipeline spec
spec:
  input:
    istio:
      enabled: true
      envoyMetrics:
        enabled: true
  output:
    otlp:
      endpoint:
        value: http://metrics.example.com:4317
  filter:
    - conditions:
        - 'IsMatch(metric.name, "^envoy_") == true and IsMatch(metric.name, ".*outlier_detection.*") == false'
```

## Limitations

Review the following limitations when constructing your processing rules.

### Explicit Context Path Required

You must specify the full context path for every attribute reference. Omitting the context (for example, writing `attributes["key"]`) is not supported.

**Correct**: `'resource.attributes["k8s.namespace.name"] == "default"'`
**Incorrect**: `'attributes["k8s.namespace.name"] == "default"'`

### No Filtering on Span Events

You cannot filter individual events within a trace span (the `spanevent` context). Filter conditions for traces apply only to the parent span itself. Data is dropped at the span level.

### Metric-Specific Functions Not Supported

Functions that are specific to the metric context, such as `HasAttrKeyOnDatapoint` and `HasAttrOnDatapoint`, are not available in the filter processor. Use general-purpose OTTL functions as an alternative:

- To replace `HasAttrKeyOnDatapoint("my.key")`, use: `ContainsValue(Keys(datapoint.attributes), "my.key")`
- To replace `HasAttrOnDatapoint("my.key", "my.value")`, use: `datapoint.attributes["my.key"] == "my.value"`

### Stability Considerations

- **Beta feature**: The underlying OTTL language is in beta state, which means syntax and function signatures may change
- **Performance impact**: Complex OTTL expressions may impact pipeline performance; test thoroughly in non-production environments first

For the most up-to-date information on supported functions and syntax, refer to the [OpenTelemetry Transformation Language documentation](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl).