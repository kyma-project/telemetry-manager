---
title: Support Istio Outgoing Communication for Metric Agent
status: Accepted
date: 2025-08-12
---

# 26. Support Istio Outgoing Communication for Metric Agent

## Context

After decoupling the Metric Agent from Metric Gateway, the Metric Agent will collect and push metrics to various backends. By default, the Metric Agent bypasses Istio’s sidecar proxy for outgoing traffic and uses Istio SDS certificates via a volume mount.

This bypass is intentional for two reasons:

1. Prometheus Scraping – Prometheus’s scrape mechanism is incompatible with the Istio sidecar proxy.
2. Outgoing TLS via Istio Certificates – Istio control plane, gateway, and Envoy metrics can be scraped without the sidecar, however, application metrics follow the Istio authentication policy: if mTLS mode is `STRICT`, Prometheus must use Istio [SDS](https://www.envoyproxy.io/docs/envoy/latest/configuration/security/secret) certificates to be able to scrape application metrics.

Current Metric Agent certificate volume mount configuration:

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

Current Metric Agent sidecar configuration to write certificates and bypass traffic redirection:

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

See [Istio documentation](https://istio.io/latest/docs/ops/integrations/prometheus/#tls-settings) for details.

### Problem Statement
The current Matric Agent setup allows only intercepting outbound traffic for the Metric Gateway on including outbound port `4317`, while bypassing all other outbound traffic interceptions.
After decoupling the Metric Agent from the Metric Gateway, we need to ensure that the Metric Agent can communicate with any in-cluster mesh backends (e.g., OTel Collector, Prometheus) and external services while maintaining Istio's security policies.

## Proposal

The Metric Agent controls sidecar interception via annotations:
- `traffic.sidecar.istio.io/includeOutboundPorts` – Ports to always intercept outbound communication (for mesh-enabled backends).
- `traffic.sidecar.istio.io/includeOutboundIPRanges` – To bypass outbound communication interception (for Prometheus scraping).

Reuse the current approach:
1. Add configured backend ports (e.g., OTel Collector, in-cluster Prometheus) to `traffic.sidecar.istio.io/includeOutboundPorts` annotation.
2. Keep IP exclusions of `traffic.sidecar.istio.io/includeOutboundIPRanges` as it is for Prometheus scraping.

Effect:
- The same port supports two flows:
  - Prometheus scrapes directly.
  - Metric Agent sends metrics via sidecar to the backend.

Metric agent would be able to push to any backend whether it is in-cluster and inside service mesh or if the backend is external to cluster.

Example: in-mesh local Prometheus backend on port `9090`:

```yaml
spec:
  template:
    metadata:
      annotations:
        traffic.sidecar.istio.io/includeOutboundPorts: "9090"
        traffic.sidecar.istio.io/includeOutboundIPRanges: ""
```

With the above configuration, the Metric Agent will be able to push metrics to the in-cluster Prometheus while still allowing the Metric Agent to scrape metrics from the targets without issues.
The Metric Pipeline reconciliation loop will handle the configuration of the Metric Agent sidecar annotations based on the configured backends, the configured **backend ports** should be added to the `traffic.sidecar.istio.io/includeOutboundPorts` annotation, regardless of whether the backend is mesh-enabled or not.

### Alternative: Istio `Sidecar` CRD
`Sidecar` CRD offers fine-grained ingress/egress control:

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

However, it only permits traffic; it does not control interception. Without annotations, all traffic is intercepted, thus breaks Prometheus scraping, hence making it unsuitable here.

## Decision

Continue using annotations (`traffic.sidecar.istio.io/includeOutboundPorts` + `traffic.sidecar.istio.io/includeOutboundIPRanges`) to control interception:

- Mesh-enabled backends use the sidecar for mTLS & policy compliance.
- Prometheus scraping remains unaffected.
- Works for mesh/non-mesh in-cluster, and external services.
- No new APIs, detection logic, or operational changes required.
- Minimal complexity, maximum compatibility.

