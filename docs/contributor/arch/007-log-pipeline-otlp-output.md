# 7. LogPipeline OTLP Output

Date: 2024-02-02

## Status

Accepted

## Context

Currently, `LogPipeline` is backed by FluentBit for log processing.
However, we want to transition to OTel Collector to consolidate logging, tracing, and metrics capabilities of the Telemetry module.
The challenge is to achieve this transition gradually without causing disruption.

## Decision

Since most existing features of `LogPipeline` are generic enough for reimplementation using OTel Collector, a new `LogPipeline` `OTLP` output will be introduced.
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

Existing FluentBit-backed features, including log enrichment, namespace, and container filtering, will be reimplemented using OTel Collector.
While OTel Collector supports a rich set of features, certain Fluent-Bit specific features, like custom filters, will not be compatible with the new `OTLP` output.
Kyma users will still be able to use the old `HTTP` output for some time, but it will be deprecated and eventually removed.

Since the new field is optional, there's no need for an API version bump.

## Consequences

Kyma users will have the flexibility to choose between FluentBit and OTel Collector-based outputs, enabling a gradual transition.
The controller must be able to handle configurations and resource management for both FluentBit and OTel Collector.
It's crucial to ensure that, in scenarios involving multiple pipelines, only one output type can be selected to maintain consistency and simplicity in the system.
This feature can be implemented using a validating webhook.

