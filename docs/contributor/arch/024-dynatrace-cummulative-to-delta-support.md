# 24. Add Support for Dynatrace Cumulative-to-Delta Metrics

Date: 2025-07-18

## Status

Proposed

## Context

The Telemetry module supports exporting metrics to Dynatrace. However, Dynatrace has [limited support](https://docs.dynatrace.com/docs/ingest-from/opentelemetry/getting-started/metrics/limitations#aggregation-temporality) for cumulative metrics and primarily accepts delta metrics. To ensure compatibility, the telemetry metric pipeline should allow converting cumulative metrics to delta format prior to export.

## Proposal

OpenTelemetry offers a [CumulativeToDelta processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/cumulativetodeltaprocessor#cumulative-to-delta-processor) that performs this transformation. We propose integrating this processor into the metric pipeline and exposing configuration options in the `MetricPipeline` API so users can enable or disable it as needed.

Two options are proposed for where this configuration could be added:

Option 1: Add `temporality` under `spec.transform`

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
    temporality: cumulative # or delta
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

Option 2: Add `temporality` under `spec.output.otlp`

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
      temporality: cumulative # or delta
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
  - It may not align well with the separation of concerns in the telemetry pipeline design.
  - The `output.otlp` API is designed to be shared across all telemetry signal types `logs`, `traces`, and `metrics`, which may lead to confusion if users expect it to apply only to metrics.

## Conclusion

Both API placement options provide valid mechanisms for configuring aggregation temporality conversion, each with trade-offs in usability, clarity, and architectural alignment.

We choose the API name `temporality (instead of more verbose alternatives like `aggregationTemporality` or `metricTemporality`) because `temporality` is a fundamental concept in how metrics are collected, stored, and reported. This highlights that temporality itself — not just a sub-aspect like aggregation or metric-specific handling— is central to the design of metrics systems.
Using the concise name `temporality` reflects that this is a core abstraction, not just an implementation detail or a modifier of another concept. It keeps the API clean and focused while preserving semantic clarity. Any user familiar with metrics will understand `temporality` in context — whereas adding prefixes like `aggregation`, `metric`, `delta`, or `cumulative` would be redundant and unnecessarily verbose.

Option 1 (placing the setting under `spec.transform`) is semantically correct and aligns well with the OpenTelemetry processor model. It treats the cumulative-to-delta conversion as a transformation step and enables potential reuse for other metric manipulations. However, it introduces complexity for users unfamiliar with transformation pipelines and may require reworking the shared transform/filter API to support multi-signal scenarios more cleanly.

Option 2 (placing the setting under `spec.output.otlp`) offers a more user-friendly and pragmatic approach. It keeps the configuration close to where users expect backend-specific behavior and avoids forcing all users to understand the transformation pipeline. While it introduces a mild violation of separation-of-concerns, this trade-off is acceptable given the clear mapping between the configuration and the needs of a specific exporter like Dynatrace.

## Decision

We will proceed with Option 2, placing the `temporality` configuration under `spec.output.otlp`. This approach prioritizes usability and clarity for the user, especially in real-world scenarios where exporters impose specific metric format constraints. To mitigate any potential confusion, the implementation should include validation and clear documentation explaining that this setting only applies to metric signals and has no effect on logs or traces.

To preserve data consistency and avoid issues with sampling or misinterpretation, the temporality processor will be placed near the end of the pipeline, immediately before the exporters. This ensures that all upstream processing steps operate on unmodified cumulative data, and the conversion to delta occurs only as a final step before export.