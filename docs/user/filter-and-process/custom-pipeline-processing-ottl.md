# Custom Pipeline Processing with OTTL

> **Note**: This feature is only available with Telemetry Manager v1.52.0 and later.

After your telemetry data has been collected, you can use the custom transform and filter feature with the [OpenTelemetry Transformation Language (OTTL)](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl) to apply advanced, fine-grained processing to your telemetry data, enriching, modifying, and filtering it before it is exported to a backend.

You can define `transform` and `filter` OTTL rules in the `spec` of your LogPipeline, MetricPipeline, and TracePipeline resources. When you define rules, the Telemetry module configures processors in the underlying OpenTelemetry Collector to execute your statements.

## Processing Order

Processing rules are always applied in a specific order. This is critical to understand when designing your rules:

1. **Transformations (transform)**: Data is first modified according to the rules in the `transform` section
2. **Filters (filter)**: The transformed data is then evaluated against the rules in the `filter` section. Any data matching a filter condition is dropped

This sequence ensures that you filter on the final, transformed state of your data. For example, if you rename an attribute in a transform rule, your filter rule must use the new name, not the original one.

## OTTL Context and Data Processing

Transform and filter operations work at different context levels depending on the telemetry data type. Currently, custom context configuration is not supported, and all operations use the lowest-level context by default:

| Pipeline Type  | Default Context | Description                                       | Requirements              |
| :------------- | :-------------- | :------------------------------------------------ | :------------------------ |
| LogPipeline    | `log`           | Operations apply to individual log records        | OTLP output only          |
| MetricPipeline | `datapoint`     | Operations apply to individual metric data points | Available for all outputs |
| TracePipeline  | `span`          | Operations apply to individual trace spans        | Available for all outputs |

> **Note**: For TracePipeline, the `span` context is used instead of `spanevent` because traces can contain spans without span events. This means you cannot currently filter or transform data within the `spanevent` context for traces.

When writing OTTL expressions, you must include the appropriate context path to access data at different levels:

- **Access current context**: `log.attributes["key"]`, `datapoint.value`, `span.name`
- **Access higher contexts**: `resource.attributes["key"]`, `scope.name`, `metric.name` (from datapoint context)
- **Access related contexts**: `span.attributes["key"]` (from spanevent context, when available)

> **Note**: By default, if an OTTL statement encounters an error, the processor logs the error and continues to process the next piece of data. This `ignore` mode prevents a single malformed data point from halting the entire pipeline.

## Transforming Telemetry Data

Use transformations to modify telemetry data as it flows through a pipeline. You can add, update, or delete attributes, change metric types, or modify span details to enrich data or conform to a specific schema.

You configure transformations in the `transform` section of a pipeline's spec.

Each transformation rule consists of:

- **statements**: A list of OTTL functions to execute
- **conditions** (optional): A list of OTTL conditions. If you provide conditions, the statements only run if at least one of the conditions is true

### Example: Add a Resource Attribute

This example adds a `deployment.environment` attribute with the value `production` to all metrics in the pipeline. Since there are no conditions, the rule applies to all data.

```yaml
# In your MetricPipeline spec
spec:
  input:
    prometheus:
      enabled: true
  output:
    otlp:
      endpoint: http://metrics.example.com:4317
  transform:
    - statements:
        - 'set(resource.attributes["deployment.environment"], "production")'
```

### Example: Conditionally Modify a Span

This example sets the status code of a trace span to `1` (Error) if its pod name matches `my-pod-name.*` and its `http.path` attribute is `/health`.

```yaml
# In your TracePipeline spec
spec:
  output:
    otlp:
      endpoint: http://traces.example.com:4317
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

### Example: Drop Metrics by Name and Value

This example drops any metric named `k8s.pod.phase` that has an integer value of `4` (which corresponds to the "Succeeded" phase).

```yaml
# In your MetricPipeline spec
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: filter-succeeded-pods
spec:
  input:
    runtime:
      enabled: true
  output:
    otlp:
      endpoint: http://metrics.example.com:4317
  filter:
    - conditions:
        - 'metric.name == "k8s.pod.phase" and datapoint.value_int == 4'
```

### Example: Drop Low-Severity Logs

This example drops log records with severity levels below INFO.

```yaml
# In your LogPipeline spec
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: LogPipeline
metadata:
  name: filter-debug-logs
spec:
  input:
    application:
      enabled: true
  output:
    otlp:
      endpoint: http://logs.example.com:4318
  filter:
    - conditions:
        - 'log.severity_number < SEVERITY_NUMBER_INFO'
```

## Limitations

Review the following limitations when constructing your processing rules.

### Explicit Context Path Required

You must specify the full context path for attributes. The system uses the lowest-level context by default:

- **LogPipeline**: Uses `log` context
- **MetricPipeline**: Uses `datapoint` context  
- **TracePipeline**: Uses `span` context

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

## Advanced Use Cases

### General resource attribute enrichment for whole cluster
<!-- TODO: Check correctness -->

Enrich all telemetry data with cluster-wide information such as environment, region, or cluster identifier:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: cluster-enrichment
spec:
  input:
    prometheus:
      enabled: true
    runtime:
      enabled: true
  output:
    otlp:
      endpoint: http://metrics.example.com:4317
  transform:
    - statements:
        # Add global cluster information
        - 'set(resource.attributes["cluster.name"], "production-eu-west-1")'
        - 'set(resource.attributes["cluster.environment"], "production")'
        - 'set(resource.attributes["cluster.region"], "eu-west-1")'
        - 'set(resource.attributes["cluster.provider"], "aws")'
        # Normalize existing region attribute if present
        - 'set(resource.attributes["cluster.region"], resource.attributes["k8s.node.region"]) where resource.attributes["k8s.node.region"] != nil'
        # Add telemetry collection timestamp
        - 'set(resource.attributes["telemetry.collected_at"], Now())'
```

### Conditional resource attribute enrichment for some workloads/namespaces
<!-- TODO: Check correctness -->

Apply different enrichment rules based on namespace or workload characteristics:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: TracePipeline
metadata:
  name: conditional-enrichment
spec:
  output:
    otlp:
      endpoint: http://traces.example.com:4317
  transform:
    # Production workloads enrichment
    - conditions:
        - 'resource.attributes["k8s.namespace.name"] matches "prod-.*"'
      statements:
        - 'set(resource.attributes["environment"], "production")'
        - 'set(resource.attributes["cost.center"], "engineering")'
        - 'set(resource.attributes["compliance.level"], "high")'
    
    # Development workloads enrichment
    - conditions:
        - 'resource.attributes["k8s.namespace.name"] matches "(dev|test|staging)-.*"'
      statements:
        - 'set(resource.attributes["environment"], "development")'
        - 'set(resource.attributes["cost.center"], "r-and-d")'
        - 'set(resource.attributes["compliance.level"], "standard")'
    
    # Critical service identification
    - conditions:
        - 'resource.attributes["k8s.deployment.name"] in ["payment-service", "user-auth", "order-processor"]'
      statements:
        - 'set(resource.attributes["service.criticality"], "high")'
        - 'set(resource.attributes["monitoring.alerting"], "pagerduty")'
    
    # Cleanup internal attributes
    - statements:
        - 'delete_matching_keys(span.attributes, "internal\\..*")'
        - 'delete_matching_keys(resource.attributes, "k8s\\.pod\\.uid")'
```

### Filter envoy metrics by outlier_detection only
<!-- TODO: Check correctness -->

Filter Istio/Envoy metrics to keep only outlier detection related metrics, reducing noise and focusing on circuit breaker behavior:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: envoy-outlier-detection
spec:
  input:
    istio:
      enabled: true
  output:
    otlp:
      endpoint: http://metrics.example.com:4317
  transform:
    # Enrich outlier detection metrics with additional context
    - conditions:
        - 'metric.name matches ".*outlier.*"'
      statements:
        - 'set(resource.attributes["circuit_breaker.monitoring"], "enabled")'
        - 'set(metric.description, Concat([metric.description, " [Circuit Breaker Monitoring]"], ""))'
  filter:
    # Keep only outlier detection related metrics
    - conditions:
        - 'metric.name matches ".*outlier.*" == false'
    
    # Drop metrics from specific non-production namespaces
    - conditions:
        - 'resource.attributes["k8s.namespace.name"] in ["kube-system", "istio-system", "monitoring"]'
    
    # Drop low-value metrics to reduce cardinality
    - conditions:
        - 'metric.name in ["envoy_cluster_manager_cds_version", "envoy_server_uptime", "envoy_server_live"]'
    
    # Drop test traffic metrics
    - conditions:
        - 'datapoint.attributes["user_agent"] == "synthetic-test"'
        - 'datapoint.attributes["request_header_x_test"] == "true"'
```