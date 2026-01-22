# Filter with OTTL

Use filters to drop unwanted telemetry data from a pipeline. Filtering helps you reduce noise, lower storage and processing costs, and focus on the data that matters most.

## Overview

You define these rules in the `filter` section of your Telemetry pipeline's `spec`.

Each rule in the `filter` list contains one or more `conditions`.

The pipeline drops any log, metric, or trace that matches **at least one** of the conditions you define. This means that multiple conditions are always combined with a logical OR. If any single condition evaluates to true, the data is dropped.

> [!TIP]
> - Filters run **after** all transformations. Your filter conditions must operate on the final, modified state of your data, not its original state.
> - This feature is based on the [OpenTelemetry Filter Processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/filterprocessor/README.md), with some limitations and differences (see [Limitations](./README.md#limitations) and [Predefined Contexts](#predefined-contexts)).

## Predefined Contexts

For each signal type, your OTTL conditions automatically operate on a predefined data context:

- **LogPipeline**: Conditions act on individual log records (context: `log`). For the list of supported field paths in this context, see [OTel Log Context](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/contexts/ottllog/README.md).
- **TracePipeline**: Conditions act on individual spans (context: `span`). For the list of supported field paths in this context, see [OTel Span Context](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/contexts/ottlspan/README.md).
- **MetricPipeline**: Conditions act on individual metric data points (context: `datapoint`). For the list of supported field paths in this context, see [OTel DataPoint Context](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/contexts/ottldatapoint/README.md).

## Example: Drop Low-Severity Logs

You can filter out `DEBUG` and `INFO` logs to reduce noise and storage costs:

```yaml
# In your LogPipeline spec
spec:
  input:
    runtime:
      enabled: true
  output:
    otlp:
      endpoint:
        value: http://logs.example.com:4317
  filter:
    - conditions:
        - 'log.severity_number < SEVERITY_NUMBER_WARN'
```

## Example: Keep Only Specific Envoy Metrics

> [!TIP]
> To start collecting envoy metrics, see [Collect Envoy Metrics](../../collecting-metrics/istio-input.md#collect-envoy-metrics).

You can keep all standard Istio metrics but filter the verbose Envoy metrics (those prefixed with `envoy_`) to retain only those related to outlier detection (circuit breaking) and host ejections in your service mesh.

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
