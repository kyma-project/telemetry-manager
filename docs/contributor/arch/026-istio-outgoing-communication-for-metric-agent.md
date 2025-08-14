---
title: Support Istio Outgoing Communication for Metric Agent
status: Accepted
date: 2025-08-12
---

# 26. Support Istio Outgoing Communication for Metric Agent

## Context

The **Metric Agent** collects and pushes metrics to various backends. By default, it **bypasses Istio’s sidecar proxy** for outgoing traffic and uses Istio SDS certificates via a volume mount.

This bypass is intentional for two reasons:

1. **Prometheus Scraping** – Prometheus’s scrape mechanism is incompatible with the Istio sidecar proxy.
2. **Outgoing TLS via Istio Certificates** – Istio control plane, gateway, and Envoy metrics can be scraped without the sidecar. Application metrics follow the Istio authentication policy: if mTLS is `STRICT`, Prometheus must use Istio SDS certificates.

Certificate volume mount configuration:

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

Sidecar configuration to write certificates and bypass traffic redirection:

```yaml
spec:
  template:
    metadata:
      annotations:
        traffic.sidecar.istio.io/includeOutboundIPRanges: ""  # no outbound interception
        proxy.istio.io/config: |  
          proxyMetadata:
            OUTPUT_CERTS: /etc/istio-output-certs
        sidecar.istio.io/userVolumeMount: '[{"name": "istio-certs", "mountPath": "/etc/istio-output-certs"}]'
```

See [Istio documentation](https://istio.io/latest/docs/ops/integrations/prometheus/#tls-settings) for details.

Previously, with the **Metric Agent** and **Metric Gateway** coupled, routing was handled centrally. After decoupling, the Metric Agent pushes directly to in-cluster backends. If the backend (e.g., OTel Collector, Prometheus, mesh-enabled service) is in the mesh, traffic must go through the sidecar for mTLS, policy, and telemetry.  Without this, traffic to mesh-enabled backends bypasses mTLS and security enforcement.

## Proposal

The Metric Agent controls sidecar interception via annotations:
- `traffic.sidecar.istio.io/includeOutboundIPRanges` – IP ranges to **bypass** interception (for Prometheus scraping).
- `traffic.sidecar.istio.io/includeOutboundPorts` – Ports to **always intercept** (for mesh-enabled backends).

Reuse the current approach:
1. Add backend ports (e.g., OTel Collector, in-cluster Prometheus) to `includeOutboundPorts`.
2. Keep IP exclusions for Prometheus scraping.

Effect:
- Same port supports two flows:
  - Prometheus scrapes directly.
  - Metric Agent sends traffic via sidecar.
- No need to detect mesh membership works for all cases.

Example: in-mesh local Prometheus on port `9090`:

```yaml
spec:
  template:
    metadata:
      annotations:
        traffic.sidecar.istio.io/includeOutboundPorts: "9090"
        traffic.sidecar.istio.io/includeOutboundIPRanges: ""
```

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

However, it only **permits** traffic; it does not control interception.  
Without annotations, all traffic is intercepted, breaking Prometheus scraping making it unsuitable here.

## Decision

Continue using annotations (`includeOutboundIPRanges` + `includeOutboundPorts`) to control interception:

- Mesh-enabled backends use the sidecar → mTLS & policy compliance.
- Prometheus scraping remains unaffected.
- Works for mesh, non-mesh in-cluster, and external services.
- No new APIs, detection logic, or operational changes required.
- Minimal complexity, maximum compatibility.

