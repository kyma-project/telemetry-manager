---
title: Support Istio Outgoing Communication for Metric Agent
status: Accepted
date: 2025-08-12
---

# 26. Support Istio Outgoing Communication for Metric Agent

## Context

By default, the Metric Agent bypasses Istio’s sidecar proxy for outgoing traffic and uses Istio SDS certificates using a volume mount.

We chose this bypass for two reasons:

1. Prometheus scraping of Istio and Envoy metrics: The metric endpoint ports of Istio and Envoy metrics are not reachable with the sidecar proxy, so the metric agent must bypass the sidecar proxy to scrape these metrics.
2. Outgoing TLS with Istio certificates: Istio control plane, gateway, and Envoy metrics can be scraped without the sidecar. However, application metrics follow the Istio authentication policy: if mTLS mode is `STRICT`, Prometheus must use Istio [SDS](https://www.envoyproxy.io/docs/envoy/latest/configuration/security/secret) certificates to scrape application metrics.

See the current Metric Agent certificate volume mount configuration:

```yaml
containers:
  - name: metric-agent
    ...
    volumeMounts:
      mountPath: /etc/prom-certs/
      name: istio-certs
volumes:
  - emptyDir:
      medium: Memory
    name: istio-certs
```

The following Metric Agent sidecar configuration writes certificates and bypasses traffic redirection:

```yaml
spec:
  template:
    metadata:
      annotations:
        traffic.sidecar.istio.io/includeOutboundIPRanges: ""  # no outbound interception
        traffic.sidecar.istio.io/includeOutboundPorts: "4317" # always intercept OTLP traffic to Metric Gateway
        proxy.istio.io/config: |  
          proxyMetadata:
            OUTPUT_CERTS: /etc/istio-output-certs
        sidecar.istio.io/userVolumeMount: '[{"name": "istio-certs", "mountPath": "/etc/istio-output-certs"}]'
```

For details, see [Istio: TLS Settings](https://istio.io/latest/docs/ops/integrations/prometheus/#tls-settings).

### Problem Statement
The current Metric Agent setup only intercepts outbound traffic for the Metric Gateway (port `4317`), while bypassing interception for all other outbound traffic.
After decoupling the Metric Agent from the Metric Gateway, we must ensure that the Metric Agent can communicate with any in-cluster mesh backends (such as OTel Collector, Prometheus) and external services while maintaining Istio's security policies.

### Tested Alternatives

1. Metric Agent with Istio sidecar proxy enabled, but without any annotations:
   With this configuration, the Metric Agent can use the sidecar proxy for outgoing traffic. This setup works for mesh-enabled backends and scraping application metrics, but breaks Prometheus scraping of Istio and Envoy metrics, because their metric endpoint ports are not reachable with the sidecar proxy.

2. Istio `Sidecar` CRD:
   The `Sidecar` CRD offers fine-grained ingress/egress control, like wildcard DNS names, ports, and IP ranges. We can use it to control the sidecar proxy behavior for the Metric Agent.

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

However, `Sidecar` CRD only permits traffic; it does not control interception. Without annotations, all traffic is intercepted, which breaks Prometheus scraping of Istio and Envoy metrics which makes it unsuitable here.
The `Sidecar` CRD does not work in combination with the Istio annotations `traffic.sidecar.istio.io/includeOutboundPorts` and `traffic.sidecar.istio.io/includeOutboundIPRanges`, because these annotations take precedence over the `Sidecar` CRD configuration.

## Proposal

The Metric Agent controls sidecar interception with the following annotations:
- `traffic.sidecar.istio.io/includeOutboundPorts` – Ports specifies the ports that always intercept outbound communication (for mesh-enabled backends).
- `traffic.sidecar.istio.io/includeOutboundIPRanges` – To bypass outbound communication interception, the annotation must have an empty value, in this way Prometheus scraping of Istio and Envoy metrics enabled.

We reuse the current approach:
1. Metric Agent adds configured backend ports (such as OTel Collector, or in-cluster Prometheus) to the `traffic.sidecar.istio.io/includeOutboundPorts` annotation.
2. Keep `traffic.sidecar.istio.io/includeOutboundIPRanges` configured as it is, because Prometheus scraping of Istio and Envoy metrics requires bypassing the proxy.

Effect:
- The same port added to the `traffic.sidecar.istio.io/includeOutboundPorts` supports two flows:
  - Prometheus scrapes application metrics directly.
  - Metric Agent sends metrics via sidecar to the backend.
- The Metric Agent can scrape Istio and Envoy metrics without issues, because the ports for these metrics are not included in the `traffic.sidecar.istio.io/includeOutboundPorts` annotation.
- The Metric Agent will be able to push to any backend, whether it is in-cluster and inside the service mesh, or external to the cluster.

See the following example for an in-mesh local Prometheus backend on port `9090`:

```yaml
spec:
  template:
    metadata:
      annotations:
        traffic.sidecar.istio.io/includeOutboundPorts: "9090"
        traffic.sidecar.istio.io/includeOutboundIPRanges: ""
```

With this configuration, the Metric Agent can push metrics to the in-cluster Prometheus, while still allowing the Metric Agent to scrape metrics from the targets without issues.
The Metric Pipeline reconciliation loop will handle the configuration of the Metric Agent sidecar annotations based on the configured backends. The configured backend ports should be added to the `traffic.sidecar.istio.io/includeOutboundPorts` annotation, regardless of whether the backend is mesh-enabled or not.

## Decision

Continue using annotations (`traffic.sidecar.istio.io/includeOutboundPorts` + `traffic.sidecar.istio.io/includeOutboundIPRanges`) to control interception:
 
- Mesh-enabled backends use the sidecar for mTLS and policy compliance.
- Prometheus scraping remains unaffected.
- Works for in-cluster services within and outside the mesh, as well as for external services.
- No new APIs, detection logic, or operational changes required.
- Minimal complexity, maximum compatibility.

