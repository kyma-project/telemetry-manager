# 22. Add Support for Dynatrace Cumulative-to-Delta Metrics

Date: 2025-07-18

## Status

Proposed

## Context

The telemetry module supports exporting metrics to Dynatrace. However, Dynatrace has [limited support](https://docs.dynatrace.com/docs/ingest-from/opentelemetry/getting-started/metrics/limitations#aggregation-temporality) for cumulative metrics and primarily accepts delta metrics. To ensure compatibility, the telemetry metric pipeline should allow converting cumulative metrics to delta format prior to export.

## Proposal

OpenTelemetry offers a [CumulativeToDelta processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/cumulativetodeltaprocessor#cumulative-to-delta-processor) that performs this transformation. We propose integrating this processor into the metric pipeline and exposing configuration options in the `MetricPipeline` API so users can enable or disable it as needed.

Two options are proposed for where this configuration could be added:

Option 1: Add `aggregationTemporality` under `spec.transform`

This places the setting within the transformation section of the pipeline, where users define other metric transformations.

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
  transform:
    aggregationTemporality: cumulative # or delta or none (default)
    rules:
      - conditions:
          - ...
          - ...
        statements:
          - ...
          - ...
  filter: {}
  output:
    otlp:
      endpoint:
        value: http://foo.bar:4317
```

- **Pros**:
    - Aligns with the nature of the operation as a transformation.
    - Supports future extensibility of the `transform` section.
    - Consistent with OpenTelemetry’s processing model.
- **Cons**:
    - Introduces API complexity for users unfamiliar with transformation rules.
    - Requires additional documentation and clarity around transform semantics.
    - The shared transform API may require rethinking for multi-signal support.

Option 2: Add `aggregationTemporality` under `spec.output.otlp`

This locates the setting within the backend-specific output configuration, directly associating it with the Dynatrace OTLP exporter.

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
  transform:
  - conditions:
    - ...
    - ...
    statements:
    - ...
    - ...
  filter: {}
  output:
    otlp:
      aggregationTemporality: cumulative # or delta or none (default)
      endpoint:
        value: http://foo.bar:4317
```

- **Pros**
  - Keeps the setting close to where the metrics are sent. 
  - Feels natural for users who expect backend-specific control in the output section.

- **Cons**
  - Adds ambiguity to the purpose of the output section, which traditionally doesn’t handle transformations. 
  - Users may not expect transformation logic to appear here. 
  - Similar to `Option 1`, it introduces complexity and documentation requirements. 
  - May not align well with the separation of concerns in the telemetry pipeline design.
  - The `output.otlp` API designed to share across all signal types, which may lead to confusion if users expect it to apply only to metrics.

## Conclusion

Both API placement options offer valid paths for enabling aggregation temporality configuration, but they come with trade-offs in clarity, usability, and architectural alignment.

Option 1 (under `spec.transform`) is more appropriate from a semantic and architectural perspective, as the conversion from cumulative to delta is clearly a data transformation. It aligns with OpenTelemetry's processor model and offers better extensibility for future transformations. However, it may require adjustments to the current transform/filter API and additional guidance for users.

Option 2 (under `spec.output.otlp`) is more intuitive for users who think in terms of backend-specific configurations but risks diluting the separation of concerns between transformation and export.

Recommendation: Proceed with Option 1 and improve user experience through clear documentation and validation. This approach better preserves architectural clarity while still providing the necessary flexibility for users targeting Dynatrace and other backends with specific temporality constraints.