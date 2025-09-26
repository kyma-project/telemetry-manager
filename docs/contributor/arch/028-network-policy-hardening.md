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
* Egress: Kubernetes API, DNS, external logging services (for example, CLS)

#### Log, Trace, Metric Gateway:
* Ingress: Metric scraping (self-monitoring and RMA), health checks, OTLP data ingested from customer workloads
* Egress: Kubernetes API, DNS, external telemetry backends (for example, CLS or Dynatrace)

#### Metric Agent:
* Ingress: Metric scraping (self-monitoring and RMA), health checks
* Egress: Kubernetes API, DNS, Kubelet, scraping customer workloads metrics, external telemetry backends (for example, CLS or Dynatrace)

#### Self-Monitor:
* Ingress: Metric scraping (self-monitoring and RMA), health checks, alert queries
* Egress: Kubernetes API, DNS, scraping module components metrics

#### Telemetry Manager:
* Ingress: Metric scraping (RMA), health checks, alertmanager webhook, admission and conversion webhooks
* Egress: Kubernetes API, DNS, self-monitor alert queries

### Current Network Policies

### Current Network Policy Configuration

Current network policies are too weak and do not meet the requirements desribed here: https://github.com/kyma-project/kyma/issues/18818.

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

The current network policies are too weak. They do not meet the requirements described in https://github.com/kyma-project/kyma/issues/18818. These policies allow all IP addresses for both incoming and outgoing traffic. They only limit ports for incoming traffic, which means they can be made stronger. However, we will still need to allow any IP address (0.0.0.0/0) in some cases for incoming and outgoing rules. External services like CLS and Dynatrace, as well as Kube API server, use different IP address ranges that we cannot predict. This makes it hard to restrict outgoing traffic by IP address. Instead, we will restrict outgoing traffic by port number.

We also decided to use the label selector `networking.kyma-project.io/metrics-scraping: allowed` not only for RMA, but also for metric agent, self-monitoring, and customer-managed Prometheus deployments. Gardener system Pods cannot be labeled in the zero-trust mode, so these Pods must be excluded from scraping.

We must test the network policies using our E2E tests to ensure they function as intended. The problem is that k3s uses Flannel as the default CNI. Real Kyma clusters typically use Calico, which behaves slightly differently. We need to find a way to run our E2E tests with Calico to accurately validate the network policies.

### What to do?

# Phase 1: Hardening Existing Network Policies

- Rename existing network policies to follow new naming conventions: `kyma-project.io--telemetry-<network-policy-name>`
- Remove health check ports from ingress rules because they have no impact.
- Remove Gardener system Pods from our scraping jobs.
- Implement cross-component network policies to allow essential services like DNS and Kubernetes API access.
- Harden telemetry manager and self-monitoring because it requires no breaking changes.
- Either expand Gardener E2E test suite to cover more scenarios or find a way to run E2E tests with Calico CNI.
- Separate self-monitoring webhook from admission webhooks in telemetry manager to allow more fine-grained ingress rules.

# Phase 2: Introduce Zero-trust Network Policies

- Implement a feature toggle in the Telemetry CR to enable/disable extra rules that harden customer-to-telemetry communication as well as RMA, cross-Kyma module communication. Because the Telemetry module already has basic network policies in place, it's illogical to use the toggle name  **enableNetworkPolicies** (because we'll still have network policies even if it's set to `false`). However, we will maintain this toggle for consistency with other Kyma modules.
- Document the required Pod labels for customer workloads to ensure proper communication with telemetry components.

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
