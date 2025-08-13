---
title: Support Istio Outgoing Communication for Metric Agent
status: Accepted
date: 2025-08-12
---

# 26. Support Istio Outgoing Communication for Metric Agent

## Context

The **Metric Agent** is a lightweight component responsible for collecting and pushing metrics to various backends. By default, it **bypasses Istio’s sidecar proxy** for outgoing communication. 
This bypass is intentional and exists for two main reasons:

1. **Direct Prometheus Scraping** – Prometheus-Receiver can scrape metrics from the agent without traffic detouring through the sidecar. This avoids unnecessary latency and complexity in scraping paths.
2. **Outgoing TLS via Istio Certificates** – Even without going through the sidecar, the Metric Agent can still leverage Istio-issued mTLS certificates for secure outgoing connections to mesh-enabled services.

Previously, when the Metric Agent and **Metric Gateway** were tightly coupled, the communication model was straightforward. The Metric Gateway would handle traffic routing to in-cluster and external backends.

However, after **decoupling** the Metric Agent from the Metric Gateway, the responsibility for directly pushing metrics to in-cluster destinations now falls to the Metric Agent itself. This creates a new requirement:

- If the backend (e.g., OTel Collector, Prometheus, or another mesh-enabled service) is **part of the Istio mesh**, the Metric Agent must **send traffic through the Istio sidecar proxy** to ensure mTLS, policy enforcement, and telemetry integration.

Without this change, traffic to mesh-enabled backends would bypass the sidecar breaking mTLS, security policy enforcement, and observability.

## Proposal

Currently, the Metric Agent relies on Istio **pod annotations** to control sidecar traffic interception:

- `traffic.sidecar.istio.io/excludeOutboundIPRanges`  
  Specifies IP ranges that the sidecar should **not intercept**. This is used to ensure that Prometheus scrapes bypass the sidecar.

- `traffic.sidecar.istio.io/includeOutboundPorts`  
  Explicitly lists ports that should **always be intercepted** by the sidecar. This allows certain outgoing connections (e.g., to the Metric Gateway) to be routed through Istio.

### Current Limitations
With the default configuration, any backend listening on a mesh-enabled port would still be bypassed if its IP range was excluded for Prometheus scraping. 
This prevents the Metric Agent from sending metrics through the sidecar if the backend is mesh-enabled.

### Proposed Adjustment
We can reuse the **existing annotation-based approach** by:

1. **Adding backend ports** (used by OTel Collector, in-cluster Prometheus, etc.) to the `includeOutboundPorts` annotation.
2. Keeping the necessary IP exclusions for Prometheus scraping intact.

This way:
- The same port can serve **two purposes**: Prometheus-Receiver can scrape it directly, while outgoing traffic to that port from the Metric Agent is routed through the sidecar.
- No need to dynamically detect whether a backend is inside or outside the mesh, the configuration works uniformly.

### Alternative: Using Istio `Sidecar` CRD
Istio also offers the `Sidecar` custom resource for **fine-grained control** over allowed ingress and egress destinations for a specific workload. It can:
- Restrict traffic to specific namespaces, services, or DNS patterns.
- Configure egress paths for workloads without modifying annotations.

For example:

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

This configuration allows the Metric Agent sidecar to send traffic to any service in the cluster, regardless of namespace.

However, the Sidecar CRD only defines what traffic is permitted; it does not control interception. Without the right annotations, all outbound traffic would be intercepted, which would break Prometheus-Receiver scraping. This makes it less practical for our specific use case, since we require selective interception.

## Decision
We will continue using annotations (`traffic.sidecar.istio.io/excludeOutboundIPRanges` and `traffic.sidecar.istio.io/includeOutboundPorts`) as the primary mechanism for controlling Istio sidecar interception for the Metric Agent.

By adding the backend service ports to the includeOutboundPorts annotation:

- Outgoing communication to mesh-enabled backends will go through the Istio sidecar proxy, ensuring mTLS and policy compliance.
- Prometheus scraping will continue to work.
- The solution works consistently for:
  - Mesh services (in-cluster)
  - Non-mesh in-cluster services 
  - External services

This approach:
- Requires no new APIs or configuration structures.
- Avoids mesh membership detection logic.
- Preserves existing operational workflows.
- Minimizes complexity while maximizing compatibility.
