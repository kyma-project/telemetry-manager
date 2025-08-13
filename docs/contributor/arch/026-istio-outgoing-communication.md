---
title: Support Istio Outgoing Communication for Metric Agent
status: Accepted
date: 2025-08-12
---

# 25. Support Istio Outgoing Communication for Metric Agent

## Context

By default, the Metric Agent bypasses Istio’s sidecar proxy for outgoing communication. This is done so that Prometheus can scrape metrics directly from the agent without going through the sidecar, while still using Istio certificates for outgoing connections.

However, after decoupling the Metric Agent from the Metric Gateway, we must ensure that the Metric Agent can push metrics to in-cluster backends (e.g., OTel Collector or Prometheus) through the Istio sidecar proxy when the backend is part of the mesh.

# Proposal

Currently, the Metric Agent uses Istio annotations to configure sidecar proxy behavior:
- `traffic.sidecar.istio.io/excludeOutboundIPRanges` — bypasses the sidecar proxy for outgoing traffic.
- `traffic.sidecar.istio.io/includeOutboundPorts` — routes outgoing traffic on specified ports through the sidecar proxy (e.g., pushing to the Metric Gateway).

This configuration should also work for any backend that is in the mesh. Therefore, we need to adjust it so that the sidecar proxy handles outgoing communication on the ports used by the backend, while still allowing Prometheus to scrape metrics from those same ports.

In addition to annotations, Istio also provides CRDs such as `Sidecar` to configure proxy behavior. The `Sidecar` CRD allows fine-grained control over ingress and egress traffic, including specifying ports, namespaces, and services (via wildcard DNS) to include or exclude.
However, it only defines what Envoy is allowed to send or receive; it does not control whether traffic is intercepted. Without annotations, all outbound traffic is intercepted by the sidecar, which would break Prometheus scraping.

Example:
```yaml
apiVersion: networking.istio.io/v1beta1
kind: Sidecar
metadata:
  name: metric-agent
  namespace: kyma-system
spec:
  workloadSelector:
    labels:
      app.kubernetes.io/name: telemetry-metric-agent
  egress:
  - hosts:
    - "*/*svc.cluster.local"
```
The example above configures the sidecar proxy to allow traffic to any service in any namespace within the cluster.

# Decision

We will continue using the annotations `traffic.sidecar.istio.io/excludeOutboundIPRanges` and `traffic.sidecar.istio.io/includeOutboundPorts` to configure the sidecar proxy for the Metric Agent.

Adding the backend ports to the `includeOutboundPorts` annotation will:
- Allow the sidecar proxy to handle outgoing communication for those ports.
- Still enable Prometheus to scrape metrics from the agent on those same ports.

This approach works for both mesh and non-mesh services, as well as external services, eliminating the need to detect whether a backend is an in-cluster mesh service. It is simple, requires no changes to existing configuration, and does not introduce additional dependencies or complexity.