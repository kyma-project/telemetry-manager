# 16. Transform and Filter support for OpenTelemetry based Pipelines

Date: 2024-10-10

## Status

Proposed

## Context

In the default setup of metric and trace pipelines, users currently cannot filter, transform, or enrich data before it is sent to the backend. To provide more flexibility, users should be able to filter data based on specific conditions and apply transformations or enrichment to the data. This solution must ensure a consistent approach across all OpenTelemetry (OTel) pipelines.

## Decision

We will implement a consolidated solution in the OpenTelemetry Collector (OTel Collector) using a single Filter and Transform Processor. This processor will use the OpenTelemetry Transformation and Transport Language (OTTL) to handle both filtering and transformation tasks. Users will be able to configure the processor as they need; and only a subset of OTTL functions will be supported, focusing on the most common and impactful use cases.

Example configuration MetricPipeline:
    
```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: metricpipeline-sample
spec:
  transform:
    conditions:
      - type == METRIC_DATA_TYPE_SUM
    statements:
      - set(description, "Sum")
  filter:
    metric:
        - type == METRIC_DATA_TYPE_NONE
    datapoint:
        - metric.name == "k8s.pod.phase" and value_int == 4
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

Example configuration TracePipeline:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: TracePipeline
metadata:
  name: tracepipeline-sample
spec:
  transform:
    statements:
        - set(status.code, 1) where attributes["http.path"] == "/health"
  filter:
    span:
      - IsMatch(resource.attributes["k8s.pod.name"], "my-pod-name.*")
  output:
    otlp:
      endpoint:
        value: ingest-otlp.services.sap.hana.ondemand.com:443
      protocol: grpc
```
### Solution

The OTTL library offers functions to validate, filter, and transform data by parsing and executing expressions. To ensure robustness and stability, we will limit the available filter and transform functions to a carefully selected subset. This subset will be curated based on common use cases, ensuring essential functionality without overcomplicating the configuration process, the OTTL library provide two category of functions, the editor functions and the converters.
The editor functions used to modify the data, while the converters are used in the OTTL condition expressions to convert the data type to be able to compare it with the expected value. The current existing pipelines using a small subset of OTTL functions like setting a value, adding or removing attributes, and filtering data based on conditions. The solution will use this subset of editor function to provide the necessary functionality to the users, the additional functions when necessary can be added.

The OTTL support 3 type of error modes to handle errors in the OTTL expression execution:
- `ignore` : The processor ignores errors returned by conditions, logs them, and continues on to the next condition.
- `slient` : The processor ignores errors returned by conditions, does not log them, and continues on to the next condition.
- `propagate` : The processor returns the error up the pipeline. This will result in the payload being dropped from the collector.

The recommended error mode is `ignore` and this will be used as default configuration.

The OTTL context will be embedded in the OTTL statements, [this is still in progress ](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/29017) and will be available in the upcoming beta version. The solution will not implement the context as configuration parameter.

To ensure data consistency and sampling efficiency, the custom OTTL transformation and filtering processor will be end of the pipeline chain before exporters.

The OTel configuration :

```yaml
service:
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
                - transform/custom-transform-processor
                - filter/custom-filter-processor
                - batch
            exporters:
                - otlp/test
```

### Risk

The OTTL library is still in its alpha phase, meaning that updates to the package may introduce breaking changes, including syntax and semantic updates or bugs. This poses a risk of disrupting running pipelines.

### Mitigation

To mitigate this risk, we suggest the following measures:
- The [beta version](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/28892) of the OTTL library will be used in the OTel Collector to ensure that the library is stable and reliable.
- New Versions with Breaking Changes: New versions with [breaking changes](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/coding-guidelines.md#breaking-changes) of the OTTL library will be rolled out with backward compatibility. This will help users adapt changes and reducing the chance of widespread disruption.
- Thorough Testing: We will maintain strict version control of the OTTL library, ensuring that each update is thoroughly tested. Automated tests will validate that existing pipelines continue to work as expected.
- Documentation: We will provide detailed documentation and practical examples of how to configure filter and transformation processors using OTTL expressions.
- Community Support: We will actively engage with the OpenTelemetry community to address any issues or concerns that arise from using the OTTL library.
- The public Go API: The breaking changes most likely will not follow deprecation policy, this may require changes in our implementation.
