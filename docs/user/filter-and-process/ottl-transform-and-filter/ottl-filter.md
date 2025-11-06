# Filter with OTTL

Use filters to drop unwanted telemetry data from a pipeline. Filtering helps you reduce noise, lower storage and processing costs, and focus on the data that matters most.

You configure filters in the `filter` section of a pipeline's spec. The filter processor drops any data point, span, or log record that matches one of your conditions.

Each filter block consists of a list of conditions. If multiple conditions are provided within a filter block, they are combined with a logical `OR` (any matching condition drops the data).

## Example: Drop Debug Logs

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

## Example: Filter Envoy metrics to outlier_detection only

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