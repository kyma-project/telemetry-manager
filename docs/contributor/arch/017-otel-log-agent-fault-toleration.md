# 17. OTel Log Agent Fault Toleration 

Date: TBD

## Status

Proposed

## Context

## Decision

### Classifying Log Loss Scenarios in an OTel Log Agent with Filelog Receiver
When setting up an OpenTelemetry (OTel) Log Agent using the Filelog Receiver, it's important to identify situations where logs might be lost and how to mitigate them.

#### Scenarios Where Data Loss Must Be Prevented:
* Temporary OTLP backend issues (e.g., spikes in retriable errors, backpressure, temporary network failures).
#### Scenarios Where Preventing Data Loss Is Beneficial:
* Collector Pod restarts during normal operations (e.g., upgrades, rescheduling to another node).
#### Scenarios Where Preventing Data Loss Is Nice-to-Have:
* Collector Pod crashes that occur unexpectedly.
#### Scenarios Where Data Loss Is Unavoidable:
* Permanent OTLP backend failures.
* Node-level failures.
* Logs not yet tailed before Collector Pod eviction.
* Logs not yet tailed before being rotated.


