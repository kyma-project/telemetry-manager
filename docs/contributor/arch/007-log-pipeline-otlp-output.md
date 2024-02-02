# 7. LogPipeline OTLP Output

Date: 2024-02-02

## Status

Accepted

## Context

Currently, `LogPipeline` is backed by Fluent Bit for log processing.
However, we want to transition to OTel Collector to consolidate the logging, tracing, and metrics capabilities of the Telemetry module.
The challenge is to achieve this transition gradually without causing disruption.

One potential solution is to introduce a new API version, allowing for the coexistence of a Fluent Bit-backed `LogPipeline` and an OTel Collector-backed `LogPipeline`. However, this approach comes with several drawbacks:

* Two controllers are needed, each responsible for a different API version. It's possible to achieve it by appending a label to the `LogPipeline` resource during creation through a mutation webhook, and filtering by this label in the respective controllers.
However, manual editing of the label remains possible.
* The Kubernetes API server allows any client to request objects at any version, making it impossible to maintain clear separation between versions.

An alternative approach involves introducing a new kind (e.g., `LogFlow`) or another API group (e.g., `logpipeline.flow.kyma-project.io`).
However, this option is less favorable as it would require a substantial effort to migrate existing resources. We would also like to keep the old names as they accurately reflect the purpose of the resource.

## Decision

Since most existing features of `LogPipeline` are generic enough for reimplementation using OTel Collector, a new `LogPipeline` `OTLP` output can be introduced.
This output option will facilitate the sending of logs to OTel Collector.

```yaml
apiVersion: telemetry.kyma-project.io/v1beta1
kind: LogPipeline
metadata:
    name: otlp-pipeline
spec:
    output:
        otlp:
            endpoint:
                value: https://backend.example.com:4317
```

Existing Fluent Bit-backed features, including log enrichment, namespace, and container filtering, will be reimplemented using OTel Collector.
While OTel Collector supports a rich set of features, certain Fluent Bit-specific features, like custom filters, will not be compatible with the new `OTLP` output.
Kyma users will still be able to use the old `HTTP` output for some time, but it will be deprecated and eventually removed.

Since the new field is optional, there's no need for an API version bump.

## Consequences

Kyma users will have the flexibility to choose between Fluent Bit and OTel Collector-based outputs, enabling a gradual transition.
The controller must be able to handle configurations and resource management for both Fluent Bit and OTel Collector.
It's crucial to ensure that, in scenarios involving multiple pipelines, only one output type can be selected to maintain consistency and simplicity in the system. That can be ensured by a validating webhook.

