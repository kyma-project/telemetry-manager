# 16. Transform and Filter Support for OpenTelemetry-based Pipelines

Date: 2024-10-10

## Status

Accepted

## Context

In the default setup of metric and trace pipelines, users currently cannot filter, transform, or enrich data before it is sent to the backend. To provide more flexibility, users should be able to filter data based on specific conditions and apply transformations or enrichment to the data. This solution must ensure a consistent approach across all OpenTelemetry (OTel) pipelines.

## Decision

We will implement a consolidated solution in the OpenTelemetry Collector (OTel Collector) using Filter and Transform processors. These processors will use the OpenTelemetry Transformation and Transport Language (OTTL) to handle both filtering and transformation tasks. Users will be able to configure the processor as needed; only a subset of OTTL functions will be supported, focusing on the most common and impactful use cases.

MetricPipeline example configuration:
    
```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: metricpipeline-sample
spec:
  transform:
    - conditions:
      - type == METRIC_DATA_TYPE_SUM
      - IsMatch(attributes["service.name"], "unknown")
      statements:
      - set(description, "Sum")
    - conditions:
      - type == METRIC_DATA_TYPE_NONE
      statements:
      - convert_sum_to_gauge() where name == "system.processes.count" and (type == METRIC_DATA_TYPE_SUM or IsMatch(attributes["service.name"], "unknown")
  filter:
      conditions:
      - metric.name == "k8s.pod.phase" and value_int == 4
      - metric.type == METRIC_DATA_TYPE_NONE
  input:
      istio:
        enabled: true
      prometheus:
        enabled: false
  output:
    otlp:
      endpoint:
        value: ingest-otlp.services.sap.hana.ondemand.com:443
```

TracePipeline example configuration:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: TracePipeline
metadata:
  name: tracepipeline-sample
spec:
  transform:
    - conditions:
      - IsMatch(span.resource.attributes["k8s.pod.name"], "my-pod-name.*")
      statements:
      - set(status.code, 1) where attributes["http.path"] == "/health"
  filter:
      conditions:
      - attributes["grpc"] == true
      - IsMatch(span.resource.attributes["k8s.pod.name"], "my-pod-name.*")
  output:
    otlp:
      endpoint:
        value: ingest-otlp.services.sap.hana.ondemand.com:443
      protocol: grpc
```
### Solution

The OTTL library offers functions to validate, filter, and transform data by parsing and executing expressions. To ensure robustness and stability, we will limit the available filter and transform functions to a carefully selected subset. This subset will be curated based on common use cases, ensuring essential functionality without overcomplicating the configuration process. The OTTL library provides two categories of functions: the [editor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/ottlfuncs#editors) functions and the [converters](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/ottlfuncs#converters).
- The editor functions are used to modify the data. The current pipelines use a small subset of OTTL functions like setting a value, adding or removing attributes, and filtering data based on conditions. We will use this subset of editor functions to provide the necessary functionality to the users. If needed, other functions can be added.
- The converter functions are used in the OTTL condition expressions to convert the data type to compare it with the expected value. They support a broad variety of data types, including XML, JSON, and string. We probably only need basic operations like string comparison, formatting, and data type checks. 


The OTTL supports three types of error modes to handle errors in the OTTL expression execution:
- `ignore` : The processor ignores errors returned by conditions, logs them, and continues on to the next condition.
- `slient` : The processor ignores errors returned by conditions, does not log them, and continues on to the next condition.
- `propagate` : The processor returns the error up the pipeline. This will result in the payload being dropped from the collector.

The recommended error mode is `ignore`, and this will be used as the default configuration.

The OTTL context will be embedded in the OTTL statements (in progress with [issue #29017](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/29017)) and will be available in the upcoming beta version. The solution will not implement the context as a configuration parameter.

The proposed API uses no context configuration for filter processors. Instead, it only allows the configuration of condition expressions. The conditions are translated to the `datapoint` context for metrics and the `spanevent` context for traces. 
Accessing higher-level context is still possible via OTTL expressions. For example, accessing the `metric` context from the `datapoint` context is possible via the expression `metric.*`, and accessing the `span` context from the `spanevent` context via the expression `span.*`.

To ensure data consistency and sampling efficiency, the custom OTTL transformation and filtering processors will be near the end of the pipeline chain, before the exporters.

See the OTel configuration:

```yaml
service:
    processors:
      transform/custom:
        error_mode: ignore
        metric_statements:
        - conditions:
          - type == METRIC_DATA_TYPE_SUM
          - IsMatch(attributes["service.name"], "unknown")
          statements:
          - set(description, "Sum")
        - conditions:
          - type == METRIC_DATA_TYPE_NONE
          statements:
          - convert_sum_to_gauge() where name == "system.processes.count"
      filter/custom:
        error_mode: ignore
        metrics:
          datapoint:
          - metric.name == "k8s.pod.phase" and value_int == 4
          - metric.type == METRIC_DATA_TYPE_NONE
    pipelines:
        metrics/test-output:
            receivers:
                - routing/test
                - forward/test
            processors:
                - filter/drop-if-input-source-runtime
                - filter/drop-if-input-source-prometheus
                - filter/drop-if-input-source-istio
                - transform/set-instrumentation-scope-kyma
                - resource/insert-cluster-name
                - resource/delete-skip-enrichment-attribute
                - transform/custom
                - filter/custom
                - batch
            exporters:
                - otlp/test
```

### Risk

The OTTL library is still in its alpha phase, meaning that updates to the package may introduce breaking changes, including syntax and semantic updates or bugs. This poses a risk of disrupting running pipelines.

### Mitigation

To mitigate this risk, we suggest the following measures:
- The [beta version](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/28892) of the OTTL library will be used in the OTel Collector to ensure that the library is stable and reliable.
- New Versions of OTTL with Breaking Changes: The breaking changes of the OTTL library are released with backward compatibility following the [breaking changes](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/coding-guidelines.md#breaking-changes) process. The backward compatibility is provided via [feature gates](https://github.com/open-telemetry/opentelemetry-collector/blob/main/featuregate/README.md). They are available in several releases. This helps us adapt to the changes and reduce the chance of widespread disruption. There are exceptional breaking changes with public Go API. These breaking changes most likely will not follow the breaking changes process. Since those changes are not end-user-faced, they may require changes in our implementation using the public API. 
- Thorough Testing: We will maintain strict version control of the OTTL library, ensuring that each update is thoroughly tested. Automated tests will validate that existing pipelines continue to work as expected.
- Documentation: We will provide detailed documentation and practical examples of how to configure filter and transformation processors using OTTL expressions.
- Community Support: We will actively engage with the OpenTelemetry community to address any issues or concerns that arise from using the OTTL library.
