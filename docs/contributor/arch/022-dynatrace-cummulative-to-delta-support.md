# 22. Add support for Dynatrace cumulative to delta metrics

Date: 2025-07-18

## Status

Proposed

## Context
Telemetry module provides support for pushing metrics to Dynatrace. However, Dynatrace has [limitations](https://docs.dynatrace.com/docs/ingest-from/opentelemetry/getting-started/metrics/limitations#aggregation-temporality) with respect to
metrics, mainly it supports only delta metrics. For this purpose, telemetry metric pipeline should provide a way to convert cumulative metrics to delta metrics before sending them to Dynatrace.


## Proposal
OpenTelemetry provides a [CumulativeToDelta processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/cumulativetodeltaprocessor#cumulative-to-delta-processor) that can be used to convert cumulative metrics to delta metrics. This processor should be added to the metric pipeline configuration before sending metrics to Dynatrace.
This option should be exposed to the user such that it can be enabled or disabled in the metric pipeline api.

The api option could be added at an appropriate place so that it is applied to all Dynatrace metric pipelines. After deliberation following is the proposed api change:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: sample
spec:
  input:
    runtime:
      enabled: true
    prometheus:
      enabled: true
    istio:
      enabled: true
  CumulativeToDelta: 
    enabled: true
  transform: {}
  filter: {}
  output:
    otlp:
      endpoint:
        value: http://foo.bar:4317
```

It is added under the `spec` section of the `MetricPipeline` resource, which is a logical place to add this option in current setup. 
The `cumulativeToDelta` field is a boolean that indicates whether the CumulativeToDelta processor should be applied to the metrics before sending them to Dynatrace. 

Adding under `filter`, `transform` sections would not be appropriate, as it is shared across all signal types. Adding it there would lead to confusion for users. We would need unnecessary additional logic to validate the signal type.

Adding it under `output` section would not be appropriate either, as this a transformation processor and not a configuration option for the output. The OTLP output is shared across all signal types so we cannot add it there.

## Conclusion

- The CumulativeToDelta processor will be added  under the `spec` section of the `MetricPipeline` resource.
- To ensure metric data consistency and sampling efficiency, the cumulativeToDelta processor will be added at the end of pipeline chain, before the exporter.
- CumulativeToDelta processor will handle all the metrics of `sum` and `histogram` type by default.