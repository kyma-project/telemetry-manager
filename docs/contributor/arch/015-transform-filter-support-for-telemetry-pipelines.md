# 15. Transform and Filter support for OpenTelemetry based Pipelines

Date: 2024-10-10

## Status

Proposed

## Context

In the default setup of Metric and Trace Pipelines, users currently lack the ability to filter or transform/enrich data before it is sent to the backend. To provide more flexibility, users should be able to filter data based on specific conditions and apply transformations or enrichment to the data. This solution must ensure a consistent approach across all OpenTelemetry (OTel) pipelines.

## Decision

We will implement a consolidated solution in the OpenTelemetry Collector (OTel Collector) using a single Filter and Transform Processor. This processor will leverage the OpenTelemetry Transformation and Transport Language (OTTL) to handle both filtering and transformation tasks. The processor will be configurable to meet user requirements, and only a subset of OTTL functions will be supported, focusing on the most common and impactful use cases.

Example configuration Metric Pipeline:
    
```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: metricpipeline-sample
spec:
  transform:
    - context: datapoint
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

Example configuration Trace Pipeline:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: TracePipeline
metadata:
  name: tracepipeline-sample
spec:
  transform:
    - context: resource
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

The OTTL library offers functions to validate, filter, and transform data by parsing and executing expressions. To ensure robustness and stability, we will limit the available filter and transform functions to a carefully selected subset. 
This subset will be curated based on common use cases, ensuring essential functionality without overcomplicating the configuration process.

### Risk

The OTTL library is still in its alpha phase, meaning that updates to the package may introduce breaking changes, including syntax and semantic updates or bugs. This poses a risk of disrupting running pipelines.

### Mitigation

To mitigate this risk:
- Thorough Testing: We will maintain strict version control of the OTTL library, ensuring that each update is thoroughly tested, automated tests will validate that existing pipelines continue to work as expected.
- Documentation: We will provide detailed documentation and practical examples of how to configure filter and transformation processors using OTTL expressions.
- New Versions with Breaking Changes: New versions with breaking changes of the OTTL library will be rolled out with backward compatibility. This will help users adapt changes and reducing the chance of widespread disruption.

### Open Questions
- How will complex transformation chains be handled efficiently?
- What are the specific failure modes if users misconfigure OTTL processors?
- What are the best practices for ensuring backward compatibility when updating OTTL versions?

