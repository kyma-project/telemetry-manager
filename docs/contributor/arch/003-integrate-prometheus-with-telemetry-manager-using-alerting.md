# 3. Integrate Prometheus With Telemetry Manager Using Alerting

Date: 2024-01-11

## Status

Accepted

## Context

As outlined in [ADR 001: Trace/Metric Pipeline status based on OTel Collector metrics](./001-otel-collector-metric-based-pipeline-status.md), our objective is to utilize a managed Prometheus instance to reflect specific telemetry flow issues (such as backpressure, data loss, backend unavailability) in the status of a telemetry pipeline custom resource (CR).
We have previously determined that both Prometheus and its configuration will be managed within the Telemetry Manager's code, aligning with our approach for managing Fluent Bit and OTel Collector.

To address the integration of Prometheus querying into the reconciliation loop, a Proof of Concept was executed.

## Decision

The results of the query tests affirm that invoking Prometheus APIs won't notably impact the overall reconciliation time. In theory, we could directly query Prometheus within the Reconcile routine. However, this straightforward approach presents a few challenges.

### Challenges

#### Timing of Invocation
Our current reconciliation strategy triggers either when a change occurs or every minute. While this is acceptable for periodic status updates, it may not be optimal when considering future plans to use Prometheus for autoscaling decisions.

#### Flakiness Mitigation
To ensure reliability and avoid false alerts, it's crucial to introduce a delay before signaling a problem. As suggested in [OTel Collector monitoring best practices](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/monitoring.md):

> Use the rate of otelcol_processor_dropped_spans > 0 and otelcol_processor_dropped_metric_points > 0 to detect data loss. Depending on requirements, set up a minimal time window before alerting to avoid notifications for minor losses that fall within acceptable levels of reliability.

If we directly query Prometheus, we would need to implement such a mechanism to mitigate flakiness ourselves.

### Solution

Fortunately, we can leverage the Alerting feature of Prometheus to address the aforementioned challenges. The proposed workflow is as follows:

#### Rendering Alerting Rules
Telemetry Manager dynamically generates alerting rules based on the deployed pipeline configuration.
These alerting rules are then mounted into the Prometheus Pod, which is also deployed by the Telemetry Manager.

#### Alert Retrieval in Reconciliation
During each reconciliation iteration, the Telemetry Manager queries the [Prometheus Alerts API](https://prometheus.io/docs/prometheus/latest/querying/api/#alerts) using `github.com/prometheus/client_golang` to retrieve information about all fired alerts.
The obtained alerts are then translated into corresponding CR statuses.

#### Webhook for Immediate Reconciliation
The Telemetry Manager exposes an endpoint intended to be invoked by Prometheus whenever there is a change in the state of alerts. To facilitate this, we can configure Prometheus to treat our endpoint as an Alertmanager instance. Upon receiving a call, this endpoint initiates an immediate reconciliation of all affected resources using the https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/builder#Builder.WatchesRawSource with https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/source#Channel.

By adopting this approach, we transfer the effort associated with expression evaluation and waiting to Prometheus.

## Consequences

The described setup involves a lot of interaction between Telemetry Manager and Prometheus, which should be sufficiently monitored.
