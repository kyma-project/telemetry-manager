---
title: Support for Filter Processor without OTTL Context Inference
status: Accepted
date: 2025-09-19
---

# 16b. Support for Filter Processor without OTTL Context Inference

## Context

The Telemetry transform and filter API was designed with the assumption that both transform and filter processors would support OTTL context inference. However, the current filter processor, in its alpha state, does not yet support OTTL context inference.
We need to decide how to handle this limitation in order to continue delivering transform and filter capabilities to our users.

## Proposal

**Filter API Implementation Using Lowest Context**  
We propose implementing a filter API that always operates at the lowest context level, such as `datapoint`, `spanevent`, or `log`. This approach enables us to provide filter capabilities to users immediately, without waiting for official OTTL context inference support in the filter processor.

Users can provide any OTTL expression with an explicit context path (similar to transform processor expressions). These expressions will be passed directly to the filter processor with a configuration that specifies the lowest context level.

The following example shows the filter API configuration in the MetricPipeline and the corresponding filter processor configuration in the OpenTelemetry Collector using the `datapoint` context level:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: metricpipeline-sample
spec:
  filter:
    conditions:
      - metric.name == "k8s.pod.phase" and datapoint.count == 4
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

Corresponding filter processor configuration in the OpenTelemetry Collector:

```yaml
processors:
  filter:
    error_mode: ignore
    metrics:
      datapoint:
        - metric.name == "k8s.pod.phase" and datapoint.count == 4
        - metric.type == METRIC_DATA_TYPE_NONE
```

Discussions with the current code owners of the filter processor indicate that support for OTTL context inference is planned, likely following the approach already established by the transform processor. However, no final API proposal is available yet.
Once official support becomes available, we can migrate to the context-less filter processor configuration without breaking existing functionality.

## Implications

Since the OpenTelemetry filter processor does not yet support context inference, users must explicitly include the appropriate context path in their OTTL expressions. This requirement increases the need for clear documentation and user guidance to ensure correct filter construction.

This approach:
- Provides immediate filter capabilities.
- Aligns with the overall design of the API for the transform and filter processors.
- Allows future migration to the official filter processor with OTTL context inference once it is available.

## Limitations

The functions `HasAttrKeyOnDatapoint` and `HasAttrOnDatapoint` can no longer be used, as they are only available within the metric context.
However, there are alternative functions that can be used instead, such as `ContainsValue(target, item)` instead of `HasAttrKeyOnDatapoint`, e.g. `ContainsValue(Keys(datapoint.attributes), "my.key")`.
The `HasAttrOnDatapoint` function can be replaced with `datapoint.attributes["my.key"] == "my.value"`.

```yaml
processors:
  filter:
    error_mode: ignore
    metrics:
      datapoint:
        - ContainsValue(Keys(datapoint.attributes), "my.key") # drops metrics containing "my.key" attribute, equal to HasAttrKeyOnDatapoint("my.key")
        - datapoint.attributes["my.key"] == "my.value"        # drops metrics containing "my.key" attribute and "my.value" value, equal to HasAttrOnDatapoint("my.key", "my.value")
```

## Conclusion
- We will implement a filter API that uses the lowest context level, requiring users to include the context path in OTTL expressions.
- The existing OTTL Validator from the transform API will be reused to ensure that filter conditions are valid and contain a context path.
- We will provide clear documentation and practical examples to help users construct filter conditions correctly.
- We will monitor the development of the filter processor, adopt OTTL context inference once available, and migrate accordingly.