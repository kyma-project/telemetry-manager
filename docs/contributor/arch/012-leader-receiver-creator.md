# 12. Leader Receiver Creator

Date: 2024-05-21

## Status

Proposed

## Context

As part of metrics collection, the Telemetry module must expose Kubernetes API server metrics. The corresponding OpenTelemetry (OTel) Collector receiver for this task is k8sclusterreceiver. However, there are some open questions:

* How can we ensure that the receiver operates in high availability mode? Should it be integrated into the metric agent (currently used only for collecting node-affine metrics), incorporated into the gateway (which typically runs multiple replicas), or deployed as a separate component?

* How can we prevent sending duplicate metrics?

## Decision
