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
     - From: Any IP<br>
       Ports: 2020, 2021, 15090(optional)
   - **Egress Rules:**
     - To: Any IP<br>
       Ports: Any

2. **OTel Log Agent**
   - **Network Policy Name:** `telemetry-log-agent`
   - **Ingress Rules:**
     - From: Any IP<br>
       Ports: 8888, 13133, 15090(optional)
   - **Egress Rules:**
     - To: Any IP<br>
       Ports: Any

3. **OTel Metric Agent**
   - **Network Policy Name:** `telemetry-metric-agent`
   - **Ingress Rules:**
     - From: Any IP<br>
       Ports: 8888, 13133, 15090(optional)
   - **Egress Rules:**
     - To: Any IP<br>
       Ports: Any

4. **OTel Log Gateway**
   - **Network Policy Name:** `telemetry-log-gateway`
   - **Ingress Rules:**
     - From: Any IP<br>
       Ports: 8888, 13133, 4318, 4317, 15090(optional)
   - **Egress Rules:**
     - To: Any IP<br>
       Ports: Any

5. **OTel Metric Gateway**
   - **Network Policy Name:** `telemetry-metric-gateway`
   - **Ingress Rules:**
     - From: Any IP<br>
       Ports: 8888, 13133, 4318, 4317, 15090(optional)
   - **Egress Rules:**
     - To: Any IP<br>
       Ports: Any

6. **OTel Trace Gateway**
   - **Network Policy Name:** `telemetry-trace-gateway`
   - **Ingress Rules:**
     - From: Any IP<br>
       Ports: 8888, 13133, 4318, 4317, 15090(optional)
   - **Egress Rules:**
     - To: Any IP<br>
       Ports: Any

7. **Self Monitor**
   - **Network Policy Name:** `telemetry-self-monitor`
   - **Ingress Rules:**
     - From: Any IP<br>
       Ports: 9090
   - **Egress Rules:**
     - To: Any IP<br>
       Ports: Any

8. **Telemetry Manager**
   - **Network Policy Name:** `telemetry-manager-manager`
   - **Ingress Rules:**
     - From: Any IP<br>
       Ports: 8080, 8081, 9443
   - **Egress Rules:**
     - To: Any IP<br>
       Ports: Any

## Decision

### Issues/questions to address

- The telemetry module already has basic network policies in place, making the `enableNetworkPolicies` toggle not logical for our use case (we still will have network policies even if it is set to false). However, we will maintain this toggle for consistency with other Kyma modules.
- Network policies have to be renamed to follow the Kyma naming convention.
- External services like CLS and Dynatrace use unpredictable IP address ranges, making IP-based egress restrictions impractical. We will address this by restricting egress traffic by port instead of IP address.
- The `networking.kyma-project.io/metrics-scraping: allowed` label selector will control ingress access for metric agents, Resource Management Agent (RMA), self-monitoring components, and customer-managed Prometheus deployments. Gardener system pods cannot be labeled with Prometheus annotations, so these pods must either be excluded from restrictions or not scraped for metrics.
- When the metric agent collects metrics from other modules, we need to establish a process to ensure all modules correctly label their pods for proper network policy enforcement.
- Istio-generated spans and access logs function correctly in the current setup. However, in restricted network policy scenarios, all pods must be properly labeled to send OTLP data to telemetry gateways.

### What to do?

- Fluent Bit and OTel Collectors need to connect to external telemetry backends (CLS, Dynatrace, etc.). We can't predict what IP ranges these external backends will use, so we must allow egress to all IPs for these components and only limit the ports. This covers all other egress traffic that the component makes (for example, we don't need separate rules for the metric agent connecting to Kubelet since all IPs are already allowed).
- Telemetry-manager and self-monitor are self-contained components and have no dependency on external services, their ingress and egress communication must be hardened.
- Telemetry gateways receive traces, logs, and metrics from customer workloads, so they must allow ingress from any IPs. Currently, they allow ingress from any IPs. In the restricted mode we can limit them by using pod label selectors. It's a breaking change that requires customer action.

### Proposed Network Policy Changes

#### Cross-component Policies (use Telemetry module-identifier label selector)

1. **Allow DNS Resolution**
   - **Policy Name:** `kyma-project.io--telemetry-allow-to-dns`
   - **Egress Rules:**
     - To: Any IP<br>
       Ports: 53 (UDP, TCP)
     - To: Namespace label matching `gardener.cloud/purpose: kube-system`, pod label matching `k8s-app: kube-dns`<br>
       Ports: 8053 (UDP, TCP)
     - To: Namespace label matching `gardener.cloud/purpose: kube-system`, pod label matching `k8s-app: node-local-dns`<br>
       Ports: 53 (UDP, TCP)

2. **Allow Kube API Server Access**
   - **Policy Name:** `kyma-project.io--telemetry-allow-to-apiserver`
   - **Egress Rules:**
     - To: Any IP<br>
       Ports: 443

#### Component-specific Policies

1. **FluentBit Agent**
   - **Network Policy Name:** `kyma-project.io--telemetry-fluent-bit`
   - **Ingress Rules:**
     - From: Pod label matching `networking.kyma-project.io/metrics-scraping: allowed` in any namespace (empty namespace selector)<br>
       Ports: 2021, 15090(optional)
   - **Egress Rules:**
     - To: Any IP<br>
       Ports: A set of ports used to connect to external logging services

2. **OTel Log Agent**
   - **Network Policy Name:** `kyma-project.io--telemetry-log-agent`
   - **Ingress Rules:**
     - From: Pod label matching `networking.kyma-project.io/metrics-scraping: allowed` in any namespace (empty namespace selector)<br>
       Ports: 8888, 15090(optional)
   - **Egress Rules:**
     - To: Any IP<br>
       Ports: A set of ports used to connect to external logging services

3. **OTel Metric Agent**
   - **Network Policy Name:** `kyma-project.io--telemetry-metric-agent`
   - **Ingress Rules:**
     - From: Pod label matching `networking.kyma-project.io/metrics-scraping: allowed` in any namespace (empty namespace selector)<br>
       Ports: 8888, 15090(optional)
   - **Egress Rules:**
     - To: Any IP<br>
       Ports: A set of ports used to connect to external metric services
     - To: Any IP<br>
       Ports: 10250
     - To: Pods matching `networking.kyma-project.io/metrics-scraping: allowed` in any namespace (empty namespace selector)<br>
       Ports: Any

4. **OTel Log Gateway**
   - **Network Policy Name:** `kyma-project.io--telemetry-log-gateway`
   - **Ingress Rules:**
     - From: Pod label matching `networking.kyma-project.io/metrics-scraping: allowed` in any namespace (empty namespace selector)<br>
       Ports: 8888, 15090(optional)
     - From: Pod label matching `networking.kyma-project.io/telemetry-otlp: allowed` in any namespace (empty namespace selector)<br>
       Ports: 4318, 4317
   - **Egress Rules:**
     - To: Any IP<br>
       Ports: A set of ports used to connect to external logging services

5. **OTel Metric Gateway**
   - **Network Policy Name:** `kyma-project.io--telemetry-metric-gateway`
   - **Ingress Rules:**
     - From: Pod label matching `networking.kyma-project.io/metrics-scraping: allowed` in any namespace (empty namespace selector)<br>
       Ports: 8888, 15090(optional)
     - From: Pod label matching `networking.kyma-project.io/telemetry-otlp: allowed` in any namespace (empty namespace selector)<br>
       Ports: 4318, 4317
   - **Egress Rules:**
     - To: Any IP<br>
       Ports: A set of ports used to connect to external metric services

6. **OTel Trace Gateway**
   - **Network Policy Name:** `kyma-project.io--telemetry-trace-gateway`
   - **Ingress Rules:**
     - From: Pod label matching `networking.kyma-project.io/metrics-scraping: allowed` in any namespace (empty namespace selector)<br>
       Ports: 8888, 15090(optional)
     - From: Pod label matching `networking.kyma-project.io/telemetry-otlp: allowed` in any namespace (empty namespace selector)<br>
       Ports: 4318, 4317
   - **Egress Rules:**
     - To: Any IP<br>
       Ports: A set of ports used to connect to external tracing services

7. **Self Monitor**
   - **Network Policy Name:** `kyma-project.io--telemetry-self-monitor`
   - **Ingress Rules:**
     - From: Pod label matching `app.kubernetes.io/name: telemetry-manager`<br>
       Ports: 9090
     - From: Pod label matching `networking.kyma-project.io/metrics-scraping: allowed` in any namespace (empty namespace selector)<br>
       Ports: 8080
   - **Egress Rules:**
     - From: Pod label matching `networking.kyma-project.io/metrics-scraping: allowed` in kyma-system namespace<br>
       Ports: Any

8. **Telemetry Manager**
   - **Network Policy Name:** `kyma-project.io--telemetry-manager`
   - **Ingress Rules:**
     - From: Any IP<br>
       Ports: 9443
     - From: Pod label matching `networking.kyma-project.io/metrics-scraping: allowed` in any namespace (empty namespace selector)<br>
       Ports: 8080
   - **Egress Rules:**
     - From: Pod label matching `app.kubernetes.io/name: self-monitor` in kyma-system namespace<br>
       Ports: 9090
