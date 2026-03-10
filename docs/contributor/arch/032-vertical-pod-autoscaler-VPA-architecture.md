---
title: Vertical Pod Autoscaler (VPA) Architecture
status: Proposed
date: 2026-02-20
---

# Vertical Pod Autoscaler (VPA) Architecture

This document proposes integrating Vertical Pod Autoscaler (VPA) with the Central OTLP Gateway and Agents to automatically adjust Pod resource requests and limits based on actual usage patterns. We evaluate two implementation approaches and recommend a strategy that ensures resource optimization while maintaining system stability.

## Context and Problem Statement

The Central OTLP Gateway and Agents are deployed as a DaemonSet with statically configured resource requests and limits. Because workload patterns vary across different nodes and over time, static resource allocation leads to the following issues:
- Under-provisioning: Pods might experience resource pressure or OOMKills during traffic spikes.
- Over-provisioning: Wasted resources when actual usage is consistently lower than allocated.
- Manual intervention: Operations teams must manually adjust resources based on observed metrics.

Vertical Pod Autoscaler (VPA) can address these issues by automatically adjusting pod resources based on historical and real-time usage data.

## Background

### VPA Architecture

VPA consists of three main components:

1. VPA Recommender: Analyzes metrics from the Metrics Server and generates resource recommendations stored in the VerticalPodAutoscaler CRD
2. VPA Updater: Evicts pods that need resource updates based on the recommendations
3. VPA Admission Controller: A mutating webhook that injects recommended resource values into pods during creation

For detailed VPA architecture, see [Kubernetes VPA Documentation](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler).

### Current State

- The central OTLP Gateway and Agents DaemonSet have a static resource configuration. For example, the gateway has the following configuration:
    - `requests.memory`: 32Mi
    - `limits.memory`: 2000Mi
    - Request-to-limit ratio: 62.5x (2000Mi / 32Mi)
- `GOMEMLIMIT` is set dynamically based on the memory limit (80% of limit)
- No automated resource scaling mechanism exists.

## Important Considerations

### Request-to-Limit Ratio

The current request-to-limit ratio of 62.5 is problematic because VPA preserves this ratio when it updates resources. For example, if VPA recommends increasing a Pod's memory request to `64Mi`, it also calculates the new limit as `4000Mi` (62.5 × 64Mi). This calculated limit is likely to exceed the memory capacity of a typical node.

Before enabling VPA, reduce the request-to-limit ratio to a more reasonable value (for example, 2-4x).

### VPA Limitations
- Limits Calculation: VPA's `maxAllowed` constraint applies only to requests, not limits. Limits are calculated from the request-to-limit ratio, which can exceed `maxAllowed` values. 
- Scale-Down Timing: VPA makes scale-down decisions based on long-term historical data (typically 8+ days), so resource reductions take time. 
- DaemonSet Updates: VPA requires Pod restarts for resource changes, which means temporary gaps in coverage for DaemonSet Pods. This applies only to clusters that don't support in-place updates (Kubernetes versions before v1.35 or clusters where the feature gate `InPlacePodVerticalScaling` is disabled).

### GOMEMLIMIT Strategy

We need to have a strategy to set `GOMEMLIMIT` when VPA manages Pod resources.
- The `GOMEMLIMIT` value must be a percentage of the memory limit that VPA sets, for example, 80% of the memory limit.
- The `GOMEMELIMT` calculation must be dynamic. That means it must change when VPA sets a new memory limit.


## Considered Options

We're evaluating two architectural approaches for implementing VPA for Central OTLP Gateway and Agents. Both options assume calculation of GOMEMLIMIT as fixed percentage value (see [GOMEMLIMIT Strategy](#gomemlimit-strategy)).

### Option 1: VPA Direct Pod Updates (Recommended)

In this option, the VPA Updater directly manages Pod resources through the VPA Admission Controller.

**Configuration:**
```yaml
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: central-gateway-vpa
spec:
  targetRef:
    apiVersion: apps/v1
    kind: DaemonSet
    name: telemetry-otlp-gateway
  updatePolicy:
    updateMode: "InPlaceOrRecreate"  # VPA evicts and recreates Pods as needed
  resourcePolicy:
    containerPolicies:
    - containerName: collector
      controlledResources: ["memory", "cpu"]
      controlledValues: RequestsAndLimits  # Only manage requests and limits
      minAllowed:
        memory: "128Mi"
        cpu: "50m"
      maxAllowed:
        memory: "1Gi"
        cpu: "1000m"
```

**Pros:**
- Stability: VPA considers Priority Class, Pod Disruption Budget, and eviction rate limits when updating Pods
- Reliability: Uses well-tested VPA components to handle complex decision logic, such as when to evict Pods.
- No Reconciliation Loops: A mutating webhook updates Pod resources, so the DaemonSet spec remains unchanged. This prevents unnecessary reconciliations.
- Kubernetes-Native: Uses standard Kubernetes autoscaling components

**Cons:**
- Visibility: DaemonSet spec doesn't reflect actual Pod resources (only visible in Pod specs)
- GOMEMLIMIT Sync: The OpenTelemetry Collector requires the `cgroupruntimeextension` to set `GOMEMLIMIT` based on memory limits, which adds complexity.

### Option 2: Reconciler-Driven Updates

This option implements a custom logic in the telemetry-manager reconciler to watch VPA recommendations and update the DaemonSet spec accordingly.

**Configuration:**
```yaml
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: central-gateway-vpa
spec:
  targetRef:
    apiVersion: apps/v1
    kind: DaemonSet
    name: telemetry-otlp-gateway
  updatePolicy:
    updateMode: "Off"  # Recommendations only, no automatic updates
  resourcePolicy:
    containerPolicies:
    - containerName: collector
      controlledResources: ["memory", "cpu"]
      minAllowed:
        memory: "128Mi"
        cpu: "50m"
      maxAllowed:
        memory: "1Gi"
        cpu: "1000m"
```

The reconciler handles the following tasks:
1. Watch VerticalPodAutoscaler CRD status
2. Compare recommendations with current DaemonSet resources
3. Update DaemonSet spec when drift exceeds threshold (for example, more than 20%).
4. Update GOMEMLIMIT based on new memory limits
5. Trigger the DaemonSet's built-in rolling update.

**Pros:**
- Visibility: DaemonSet spec always reflects actual pod resources
- GOMEMLIMIT Sync: Can automatically update `GOMEMLIMIT` based on new memory limits
- Controlled Rollout: Uses DaemonSet's `maxUnavailable` setting for controlled updates

**Cons:**
- Complexity: Must implement update decision logic (when, how much, under what conditions)
- Maintenance Burden: Need to maintain and test custom update logic
- Potential Conflicts: Risk of reconciliation loops if not carefully designed
- Redundant Logic: Duplicates functionality that VPA already provides

## Decision

We will go with Option 1: VPA Direct Pod Updates. The reconciler creates the VPA resource when it creates the relevant DaemonSet.

Rationale:
1. Lower Complexity: Uses tested VPA components rather than implementing custom logic
2. Better Stability: VPA's built-in safeguards (rate limiting, PDB awareness) reduce risk
3. Faster Implementation: Requires only VPA configuration, not code changes
4. Standard Solution: Aligns with Kubernetes best practices and community patterns
5. Acceptable Trade-offs: The visibility limitation is acceptable because monitoring tools can observe actual Pod resources

Configuration Strategy:
- Use `updateMode: "InPlaceOrRecreate"` for automatic Pod updates
- Use `controlledValues: RequestsAndLimits` to set both requests and limits.
- Set `GOMEMLIMIT` dynamically using the [OpenTelemetry Collector cgroupruntimeextension](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/cgroupruntimeextension), which sets `GOMEMLIMIT` at runtime based on available memory. We set it to use a percentage of the memory limit, for example 80%, to ensure it scales with VPA recommendations.
- Configure reasonable `minAllowed` and `maxAllowed` boundaries
- Document that actual Pod resources may differ from DaemonSet spec
- The reconciler manages the VerticalPodAutoscaler resource lifecycle. It creates the VPA instance along with other resources and deletes it when the other resources are deleted. The reconciler does not update the VPA spec after creation. VPA is solely responsible for managing Pod resources based on its recommendations.


## Consequences

### Positive Consequences

- Automated Resource Optimization: Pods are automatically sized based on actual usage
- Reduced OOMKills: VPA increases resources before memory pressure occurs
- Cost Efficiency: Resources are reclaimed when usage decreases (with some delay)
- Operational Simplicity: No manual resource tuning required
- Production Proven: VPA is widely used in production Kubernetes clusters

### Negative Consequences

- Monitoring Complexity: Need to monitor actual Pod resources, not just the DaemonSet spec
- Documentation Requirement: Need to document that the DaemonSet spec doesn't reflect reality
- Coverage Gaps: On clusters without in-place update support, Pod restarts during resource updates can cause brief monitoring gaps
- Scale-Down Delay: Resource reductions take days due to VPA's conservative approach

### Known Limitations

Consider the following inherent VPA limitations when you use it with the Central OTLP Gateway and Agents:

1. **Resource Availability**: VPA recommendations may exceed available node resources, causing Pods to remain pending. To mitigate this, we can use the Cluster Autoscaler or configure appropriate `maxAllowed` limits.
2. **HPA Incompatibility**: VPA cannot be used with Horizontal Pod Autoscaler (HPA) on the same resource metrics (CPU or memory). This is not a concern for DaemonSets, which don't support HPA.
3. **Admission Webhook Conflicts**: VPA's admission controller may conflict with other mutating admission webhooks depending on webhook configuration and ordering.
4. **Out-of-Memory Handling**: While VPA reacts to most OOM events, it cannot handle all out-of-memory scenarios and may not prevent all OOMKills. To scale Pods that are OOMed, the Pod must run long enough for the VPA Recommender to collect metrics.
5. **Multiple VPA Resources**: Configuring multiple VPA resources targeting the same pod results in undefined behavior. Ensure only one VPA resource targets the Central OTLP Gateway and Agent DaemonSet.
6. **Recommendations Without Controller**: VPA cannot update resources for standalone Pods not managed by a controller (Deployment, DaemonSet, StatefulSet, etc.).

### VPA MaxAllowed Strategy

1. Calculate based on node capacity:
   To prevent pending Pods and leave headroom for burstable workloads, configure `maxAllowed` values based on a percentage of the smallest node's memory capacity, such as 75%. For example, if five DaemonSets run on a 32Gi node, the calculation is: 32Gi × 0.75 ÷ 5 ≈ 4.8Gi per Pod.
2. Consider workload patterns:
   Analyze historical usage patterns to set `maxAllowed` values that accommodate typical spikes while preventing excessive resource allocation. The DaemonSets run on every node, one misbehaving DaemonSet can exhaust all nodes.
3. Set based on Application Architecture:
   Consider the current application limits, profile application to find the "knee of the curve" where additional resources provide diminishing returns, and set `maxAllowed` values around that point to optimize cost-performance balance. For example, if the current limit is 2Gi and usage rarely exceeds 1Gi, setting `maxAllowed` to 1.5Gi may be appropriate.
4. Account for QoS and resource ratios:
   If we set the limit to 2× the request and VPA recommends a 10 GiB request, VPA sets a 20 GiB limit, which is too high. Base maxAllowed calculations on the resulting limit, not the request.
5. Iteratively adjust:
   Start with conservative `maxAllowed` values and monitor VPA recommendations and Pod behavior. Adjust `maxAllowed` based on observed usage patterns and resource availability.