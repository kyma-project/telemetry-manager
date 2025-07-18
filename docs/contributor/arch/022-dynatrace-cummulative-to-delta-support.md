# 22. Add Support for Dynatrace Cumulative-to-Delta Metrics

Date: 2025-07-18

## Status

Proposed

## Context

The telemetry module provides support for pushing metrics to Dynatrace. However, Dynatrace has [limitations](https://docs.dynatrace.com/docs/ingest-from/opentelemetry/getting-started/metrics/limitations#aggregation-temporality) with respect to metric ingestion; specifically, it primarily supports only delta metrics. Therefore, the telemetry metric pipeline should provide a mechanism to convert cumulative metrics into delta metrics before sending them to Dynatrace.

## Proposal

OpenTelemetry provides a [CumulativeToDelta processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/cumulativetodeltaprocessor#cumulative-to-delta-processor) that can be used to convert cumulative metrics into delta metrics. This processor should be incorporated into the metric pipeline configuration prior to exporting metrics to Dynatrace. This option should be exposed to users so that it can be enabled or disabled through the metric pipeline API.

The API option could be added at an appropriate location to ensure it applies uniformly to all Dynatrace metric pipelines. Following deliberation, the proposed API change is as follows:

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
  cumulativeToDelta:
    enabled: true
  transform: {}
  filter: {}
  output:
    otlp:
      endpoint:
        value: http://foo.bar:4317
```

It is added under the `spec` section of the `MetricPipeline` resource, which is a logical placement within the current configuration. The `cumulativeToDelta` has a field `enabled`, is a boolean indicating whether the CumulativeToDelta processor should be applied to the metrics before they are sent to Dynatrace.

Placing this field under the `filter` or `transform` sections would be inappropriate, as those sections are shared across all signal types. Doing so would cause user confusion and require unnecessary additional logic to validate the signal type.

Adding it under the `output` section would also be unsuitable, as this field relates to a transformation processor, not a configuration option for output. Moreover, the OTLP output is shared across all signal types, making it an improper location for this setting.

## Conclusion
- The CumulativeToDelta processor will be added under the `spec` section of the `MetricPipeline` resource.
- To ensure metric data consistency and sampling efficiency, the CumulativeToDelta processor will be added at the end of the pipeline chain, immediately before the exporter.
- The CumulativeToDelta processor will, by default, handle all metrics of the `sum` and `histogram` types.
