---
title: Network Policy Hardening
status: Accepted
date: 2025-09-18
---

# 28. Network Policy Hardening

## Context

### Communication Flow Analysis

#### FluentBit, Log Agent:
* Ingress: Metric scraping (self-monitoring and RMA) and health checks
* Egress: Kubernetes API, DNS, external logging services (e.g., CLS)

#### Log, Trace, Metric Gateway:
* Ingress: Metric scraping (self-monitoring and RMA), health checks, OTLP data ingested by customer workloads
* Egress: Kubernetes API, DNS, external telemetry backends (e.g., CLS, Dynatrace)

#### Metric Agent:
* Ingress: Metric scraping (self-monitoring and RMA), health checks
* Egress: Kubernetes API, DNS, Kubelet, scraping customer workloads metrics, external telemetry backends (e.g., CLS, Dynatrace)

#### Self-Monitor:
* Ingress: Metric scraping (self-monitoring and RMA), health checks, alert queries
* Egress: Kubernetes API, DNS, scraping module components metrics

#### Telemetry Manager:
* Ingress: Metric scraping (RMA), health checks, alertmanager webhook, admission and conversion webhooks
* Egress: Kubernetes API, DNS, self-monitor alert queries

### Current Network Policies

### Current Network Policy Configuration

1. **FluentBit Agent**
   - **Network Policy Name:** `telemetry-fluent-bit`
   - **Ingress Rules:**
     - From: Any IP
       Ports: 2020, 2021, 15090(optional)
   - **Egress Rules:**
     - To: Any IP
       Ports: Any

2. **OTel Log Agent**
   - **Network Policy Name:** `telemetry-log-agent`
   - **Ingress Rules:**
     - From: Any IP
       Ports: 8888, 13133, 15090(optional)
   - **Egress Rules:**
     - To: Any IP
       Ports: Any

3. **OTel Metric Agent**
   - **Network Policy Name:** `telemetry-metric-agent`
   - **Ingress Rules:**
     - From: Any IP
       Ports: 8888, 13133, 15090(optional)
   - **Egress Rules:**
     - To: Any IP
       Ports: Any

4. **OTel Log Gateway**
   - **Network Policy Name:** `telemetry-log-gateway`
   - **Ingress Rules:**
     - From: Any IP
       Ports: 8888, 13133, 4318, 4317, 15090(optional)
   - **Egress Rules:**
     - To: Any IP
       Ports: Any

5. **OTel Metric Gateway**
   - **Network Policy Name:** `telemetry-metric-gateway`
   - **Ingress Rules:**
     - From: Any IP
       Ports: 8888, 13133, 4318, 4317, 15090(optional)
   - **Egress Rules:**
     - To: Any IP
       Ports: Any

6. **OTel Trace Gateway**
   - **Network Policy Name:** `telemetry-trace-gateway`
   - **Ingress Rules:**
     - From: Any IP
       Ports: 8888, 13133, 4318, 4317, 15090(optional)
   - **Egress Rules:**
     - To: Any IP
       Ports: Any

7. **Self Monitor**
   - **Network Policy Name:** `telemetry-self-monitor`
   - **Ingress Rules:**
     - From: Any IP
       Ports: 9090
   - **Egress Rules:**
     - To: Any IP
       Ports: Any

8. **Telemetry Manager**
   - **Network Policy Name:** `telemetry-manager-manager`
   - **Ingress Rules:**
     - From: Any IP
       Ports: 8080, 8081, 9443
   - **Egress Rules:**
     - To: Any IP
       Ports: Any

## Decision

### Issues/questions to address

- Our network policies (loose) are already present, so `enableNetworkPolicies` toggle does not make sense for us.
- No egress rules, too loose. Only ingress ports are limited at the moment.
- There is a need to define cross-workload policies applied based on labels. For example, every Telemetry module component should be able to communicate with the API server, DNS, and other essential services. And such a policy have to be defined in a single place, not per each workload. Health checks?
- Egress when reaching external services (CLS, Dynatrace, etc.).
- RMA/Prometheus scraping.
- Telemetry module collecting metrics from other modules?

### What to do?

- Fluent Bit and OTel Collectors need to connect to external telemetry backends (CLS, Dynatrace, etc.). We can't predict what IP ranges these external backends will use, so we must allow egress to all IPs for these components and only limit the ports. This covers all other egress traffic that the component makes (for example, we don't need separate rules for the metric agent connecting to Kubelet since all IPs are already allowed).
- Telemetry-manager and self-monitor are self-contained components and have no dependency on external services, their ingress and egress communication must be hardened.
- Telemetry gateways receive traces, logs, and metrics from customer workloads, so they must allow ingress from any IPs. Currently, they allow ingress from any IPs. In the restricted mode we can limit them by using pod label selectors. It's a breaking change that requires customer action.

### Proposed Network Policy Changes

#### Cross-component Policies (use Telemetry module-identifier label selector)

1. **Allow DNS Resolution**
   - **Policy Name:** `kyma-project.io--telemetry-allow-to-dns`
   - **Egress Rules:**
     - To: Any IP
       Ports: 53 (UDP, TCP)
     - To: Namespace label matching `gardener.cloud/purpose: kube-system`, pod label matching `k8s-app: kube-dns`
       Ports: 8053 (UDP, TCP)
     - To: Namespace label matching `gardener.cloud/purpose: kube-system`, pod label matching `k8s-app: node-local-dns`
       Ports: 53 (UDP, TCP)

2. **Allow Kube API Server Access**
   - **Policy Name:** `kyma-project.io--telemetry-allow-to-apiserver`
   - **Egress Rules:**
     - To: Any IP
       Ports: 443

#### Component-specific Policies

1. **FluentBit Agent**
   - **Network Policy Name:** `kyma-project.io--telemetry-fluent-bit`
   - **Ingress Rules:**
     <!--- From: Any IP
       Ports: 2020-->
     - From: Pod label matching `networking.kyma-project.io/metrics-scraping: allowed`
       Ports: 2021, 15090(optional)
   - **Egress Rules:**
     - To: Any IP
       Ports: A set of ports used to connect to external logging services

2. **OTel Log Agent**
   - **Network Policy Name:** `kyma-project.io--telemetry-log-agent`
   - **Ingress Rules:**
     <!--- From: Any IP
       Ports: 13133-->
     - From: Pod label matching `networking.kyma-project.io/metrics-scraping: allowed`
       Ports: 8888, 15090(optional)
   - **Egress Rules:**
     - To: Any IP
       Ports: A set of ports used to connect to external logging services

3. **OTel Metric Agent**
   - **Network Policy Name:** `kyma-project.io--telemetry-metric-agent`
   - **Ingress Rules:**
     <!--- From: Any IP
       Ports: 13133-->
     - From: Pod label matching `networking.kyma-project.io/metrics-scraping: allowed`
       Ports: 8888, 15090(optional)
   - **Egress Rules:**
     - To: Any IP
       Ports: A set of ports used to connect to external metric services
     <!--- To: Any IP
       Ports: 10255-->
     - To: Pods matching `networking.kyma-project.io/metrics-scraping: allowed`
       Ports: Any

4. **OTel Log Gateway**
   - **Network Policy Name:** `kyma-project.io--telemetry-log-gateway`
   - **Ingress Rules:**
     <!--- From: Any IP
       Ports: 13133-->
     - From: Pod label matching `networking.kyma-project.io/metrics-scraping: allowed`
       Ports: 8888, 15090(optional)
     - From: Pod label matching `networking.kyma-project.io/telemetry-otlp: allowed`
       Ports: 4318, 4317
   - **Egress Rules:**
     - To: Any IP
       Ports: A set of ports used to connect to external logging services

5. **OTel Metric Gateway**
   - **Network Policy Name:** `kyma-project.io--telemetry-metric-gateway`
   - **Ingress Rules:**
     <!--- From: Any IP
       Ports: 13133-->
     - From: Pod label matching `networking.kyma-project.io/metrics-scraping: allowed`
       Ports: 8888, 15090(optional)
     - From: Pod label matching `networking.kyma-project.io/telemetry-otlp: allowed`
       Ports: 4318, 4317
   - **Egress Rules:**
     - To: Any IP
       Ports: A set of ports used to connect to external metric services

6. **OTel Trace Gateway**
   - **Network Policy Name:** `kyma-project.io--telemetry-trace-gateway`
   - **Ingress Rules:**
     <!--- From: Any IP
       Ports: 13133-->
     - From: Pod label matching `networking.kyma-project.io/metrics-scraping: allowed`
       Ports: 8888, 15090(optional)
     - From: Pod label matching `networking.kyma-project.io/telemetry-otlp: allowed`
       Ports: 4318, 4317
   - **Egress Rules:**
     - To: Any IP
       Ports: A set of ports used to connect to external tracing services

7. **Self Monitor**
   - **Network Policy Name:** `telemetry-self-monitor`
   - **Ingress Rules:**
     - From: Pod label matching `app.kubernetes.io/name: telemetry-manager`
       Ports: 9090
   - **Egress Rules:**
     - From: Pod label matching `networking.kyma-project.io/metrics-scraping: allowed`
       Ports: Any

8. **Telemetry Manager**
   - **Network Policy Name:** `telemetry-manager-manager`
   - **Ingress Rules:**
     <!--- From: Any IP
       Ports: 8081, 9443-->
     - From: Any IP
       Ports: 9443
     - From: Pod label matching `networking.kyma-project.io/metrics-scraping: allowed`
       Ports: 8080
   - **Egress Rules:**
     - From: Pod label matching `app.kubernetes.io/name: self-monitor`
       Ports: 9090

## Consequences

What becomes easier or more difficult to do and any risks introduced by the change that will need to be mitigated.
