---
title: Support for Filter Processor without OTTL Context Inference
status: Accepted
date: 2025-09-19
---

# 16b. Support for Filter Processor without OTTL Context Inference

## Context

The Telemetry Transform and Filter API was designed with the assumption that both transform and filter processors would support OTTL context inference. However, the current filter processor, in its alpha state, does not yet support OTTL context inference.
We need to decide how to handle this limitation in order to continue delivering transform and filter capabilities to our users.

## Proposal

**Filter API Implementation Using Lower Context**  
We propose implementing a Filter API that always operates at a lower context level, such as `datapoint`, `spanevent`, or `log`. This approach enables us to provide Filter capabilities to users immediately, without waiting for official OTTL context inference support in the FilterProcessor.

Users can provide any OTTL expression with an explicit context path (similar to TransformProcessor expressions). These expressions will be passed directly to the FilterProcessor with a configuration that specifies a lower context level.

Following example show Filter-API configuration using the `datapoint` context level in the final OpenTelemetry configuration:**

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

Corresponding FilterProcessor configuration in the OpenTelemetry Collector:

```yaml
processors:
  filter:
    error_mode: ignore
    metrics:
      datapoint:
        - metric.name == "k8s.pod.phase" and datapoint.count == 4
        - metric.type == METRIC_DATA_TYPE_NONE
```

Discussions with the current code owners of the FilterProcessor indicate that support for OTTL context inference is planned, likely following the approach already established by the TransformProcessor. However, no final API proposal is available yet.
Once official support becomes available, we can migrate to the context-less FilterProcessor configuration without breaking existing functionality.

## Implications

Since the OpenTelemetry FilterProcessor does not yet support context inference, users must explicitly include the appropriate context path in their OTTL expressions. This requirement increases the need for clear documentation and user guidance to ensure correct filter construction.

This approach:
- Provides immediate Filter capabilities.
- Aligns with the overall design of the Telemetry Transform and Filter API.
- Allows future migration to the official FilterProcessor with OTTL context inference once it is available.

## Conclusion
- We will implement a Filter API that uses a lower context level, requiring users to include the context path in OTTL expressions.
- The existing OTTL Validator from the Transform API will be reused to ensure that filter conditions are valid and contain a context path.
- We will provide clear documentation and practical examples to help users construct filter conditions correctly.
- We will monitor the development of the FilterProcessor, adopt OTTL context inference once available, and migrate accordingly.