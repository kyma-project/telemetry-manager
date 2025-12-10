---
title: Migration from Batch Processor to Exporter Helper Batching
status: Proposed
date: 2025-12-09
---

# 29: Migration from Batch Processor to Exporter Helper Batching

## Context

The OpenTelemetry Collector's batchprocessor has known limitations and is planned for deprecation ([opentelemetry-collector#13582](https://github.com/open-telemetry/opentelemetry-collector/issues/13582)). The most significant limitation is its inability to propagate backpressure to clients due to its asynchronous behavior, forcing operators to choose between supporting batching or backpressure, but not both simultaneously.

The exporterhelper package integrates a new batching solution into exporters [see issue #8122](https://github.com/open-telemetry/opentelemetry-collector/issues/8122). This solution provides both batching capabilities and proper backpressure propagation.

### Current Situation

The testing setup consists of:

- One primary OTel Collector that receives telemetry from clients
- One healthy backend OTel Collector receiving forwarded telemetry

Backpressure is simulated by configuring a fanout pipeline with the primary OTel Collector sending to the healthy OTel Collector and an additional unreachable endpoint.

### Results

Testing confirmed that the exporterhelper approach properly handles backpressure:

With exporterhelper (new approach):

```
otelcol_exporter_enqueue_failed_metric_points_total = 333
otelcol_exporter_queue_size = 4
otelcol_receiver_refused_metric_points_total = 333
```

With batchprocessor (current approach):

```
otelcol_exporter_enqueue_failed_metric_points_total = 126
otelcol_exporter_queue_size = 4
otelcol_receiver_refused_metric_points_total = 0
```

The `receiver_refused_*` metric demonstrates that exporterhelper successfully propagates backpressure to the receiver, so clients can receive retryable errors and implement proper retry logic.

### Identified Challenge: Fanout Backpressure

When using a fanout configuration with multiple exporters, backpressure from any single unhealthy backend affects data delivery to all backends, causing:

- Significantly slower telemetry delivery to healthy backends
- Potential data loss when client queues fill up
- Duplicated telemetry appearing in the healthy backend due to client retries
- Overall system degradation despite having healthy backends available

## Decision

We will migrate from `batchprocessor` to the exporterhelper's built-in batching capabilities for all exporter configurations. During this migration, we will contact the OpenTelemetry community to discuss and potentially contribute a solution.
The [design document](https://docs.google.com/document/d/1uxnn5rMHhCBLP1s8K0Pg_1mAs4gCeny8OWaYvWcuibs) proposed a `drop_on_error` configuration in the exporter helper package so users can select which pipelines should not propagate backpressure. However, there are currently no updates on this proposal's progress. We will contact an OpenTelemetry maintainer to discuss this further.

Currently, the migration plan includes the following steps to mitigate the fanout backpressure issue:

### 1. Backpressure Isolation: Custom Processor/Connector for Primary/Secondary Backend Designation

We will implement a custom processor or connector that designates primary and secondary backends:

- **Primary backends**: Backpressure propagates normally when these backends are unhealthy.
- **Secondary backends**: Operate asynchronously without propagating backpressure to the main pipeline.
- **API exposure**: Users can configure primary backends through the Telemetry CR.

An example configuration for the custom processor may look like this:

```yaml
processors:
  fanout:
    primary_exporters: [ otlp/backend ]
    secondary_exporters: [ otlp/backend-2]

exporters:
  otlp/backend:
    endpoint: backend:4317
    sending_queue:
      enabled: true
      queue_size: 500
    retry_on_failure:
      enabled: true

  otlp/backend-2:
    endpoint: unhealthy-backend:4317
    sending_queue:
      enabled: true
      queue_size: 500
    retry_on_failure:
      enabled: true

service:
  pipelines:
    traces:
      receivers: [ otlp ]
      processors: [ fanout ]
      exporters: [ otlp/backend, otlp/backend-2]
```

### 2. Deduplication: Custom Processor for UUIDv5-based Deduplication

To handle potential duplicate telemetry in secondary backends resulting from client retries, we will implement a custom processor that generates UUIDv5 identifiers for each telemetry item based on the following fields:

- `timestamp`
- `k8s.pod.name`
- `k8s.node.name`
- `log.body` (for logs)
- `spanID` (for traces)
- `metric.name` (for metrics)

This processor ensures idempotent delivery so that backends can recognize and discard duplicates.

### 3. Monitoring Implementation

Our self monitor component monitors the following metrics:

1. **Queue Health**: `otelcol_exporter_queue_size / otelcol_exporter_queue_capacity` - Alert when threshold indicates
   backend unavailability
2. **Send Failures**: `otelcol_exporter_send_failed_*` - Track failed deliveries
3. **Enqueue Failures**: `otelcol_exporter_enqueue_failed_*` - Identify queue saturation
4. **Backpressure Events**: `otelcol_receiver_refused_*_total` - Monitor client-visible backpressure
5. **Batch Efficiency**: `otelcol_exporter_queue_batch_send_size_bucket` - Optimize batch sizes

## Consequences

### Positive

1. Clients can implement retry logic when receivers return unavailable status.
2. Aligns with OpenTelemetry's strategic direction before batchprocessor deprecation.
3. Secondary backend failures won't impact primary backend telemetry delivery.

### Negative

1. Custom processor/connector requires development and ongoing maintenance
2. If the primary backend fails, all backends experience degraded performance
3. Secondary backends may lose data during extended primary backend outages
4. If multiple primary backends are needed to deliver different telemetry to different backends, backpressure in one primary backend affects all other primary backends.

### Risks and Mitigations

| Risk                                       | Mitigation                                                              |
|--------------------------------------------|-------------------------------------------------------------------------|
| Custom component introduces bugs           | Comprehensive testing suite, gradual rollout                            |
| Secondary backends receive incomplete data | Accept as trade-off for isolation                                       |
| Performance overhead from custom component | Performance testing as part of next steps, optimize based on benchmarks |

## Next Steps

1. Investigate behavior for pull-based receivers when backpressure is propagated from the exporter:
    - filelogreceiver - has a `retry_on_failure` configuration
    - prometheusreceiver
    - k8sclusterreceiver
    - kubeletstatsreceiver
2. Contact the CLS maintainer to explore how CLS handles deduplication.
3. Contact an OpenTelemetry community member to validate the approach and gather feedback.
4. Design and implement the primary/secondary fanout processor or connector.
5. Design and implement the UUIDv5-based deduplication processor.
6. Determine optimal configurations for batching, queueing, and retry parameters based on queue_size constraints.
7. Create a phased rollout plan with rollback procedures.
8. Update operational runbooks and configuration guides.

## Alternatives Considered

### 1. Use `block_on_overflow` for Secondary Pipelines

Configure secondary pipelines with `block_on_overflow` to block threads instead of returning errors.

**Rejected because:**

- Dependent on client timeout configuration
- No telemetry delivery at all if clients don't implement timeouts
- Still doesn't provide true isolation

### 2. Route Secondary Pipelines Through Batch Processor for Asynchronous Behavior

Use the routing connector to send secondary pipelines through the deprecated batchprocessor.

**Rejected because:**

- Only a temporary solution until batchprocessor removal
- Doesn't align with the long-term OpenTelemetry direction
- Would require another migration in the future

### 3. Accept Fanout Backpressure

Keep the current fanout behavior without modification.

**Rejected because:**

- Unacceptable operational impact during partial failures
- High risk of data loss and service degradation
- Poor user experience during common failure scenarios



## References

- [Batch Processor Deprecation Issue](https://github.com/open-telemetry/opentelemetry-collector/issues/13582)
- [Exporter Helper Batching Option Issue](https://github.com/open-telemetry/opentelemetry-collector/issues/8122)
- [Use Case Discussion](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/31775)
- [Exporter Helper Package Documentation](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/exporterhelper)
- [Exporter Helper Design Document](https://docs.google.com/document/d/1uxnn5rMHhCBLP1s8K0Pg_1mAs4gCeny8OWaYvWcuibs)
