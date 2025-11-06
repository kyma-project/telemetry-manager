# Transform and Filter Telemetry Data with OTTL

> **Note**: This feature is only available with Telemetry Manager v1.52.0 and later.

After your telemetry data has been collected, you can use the custom transform and filter feature with the [OpenTelemetry Transformation Language (OTTL)](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/README.md) to apply advanced, fine-grained processing to your telemetry data, enriching, modifying, and filtering it before it is exported to a backend.

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

> For more details on the underlying implementation details of context inference, see [OTel Transform Processor Context Inference](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/transformprocessor/README.md#context-inference).

Error handling: If a statement fails (for example, referencing a missing attribute), the processor logs the error and continues (ignore mode). One bad record does not stop the pipeline.

These rules ensure predictable behavior without additional context configuration.

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

For the most up-to-date information on supported functions and syntax, refer to the [OpenTelemetry Transformation Language documentation](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/README.md).