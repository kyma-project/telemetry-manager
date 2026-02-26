---
title: Vertical Pod Autoscaler (VPA) Architecture
status: Proposed
date: 2026-02-20
---

# Vertical Pod Autoscaler (VPA) Architecture

This document proposes integrating Vertical Pod Autoscaler (VPA) with the Central OTLP Gateway to automatically adjust pod resource requests and limits based on actual usage patterns. We evaluate two implementation approaches and recommend a strategy that ensures resource optimization while maintaining system stability.

## Context and Problem Statement

The Central OTLP Gateway is deployed as a DaemonSet with statically configured resource requests and limits. As workload patterns vary across different nodes and over time, static resource allocation leads to:
- Under-provisioning: Pods may experience resource pressure or OOMKills during traffic spikes
- Over-provisioning: Wasted resources when actual usage is consistently lower than allocated
- Manual intervention: Operations teams must manually adjust resources based on observed metrics

Vertical Pod Autoscaler (VPA) can address these issues by automatically adjusting pod resources based on historical and real-time usage data.

## Background

### VPA Architecture

VPA consists of three main components:

1. VPA Recommender: Analyzes metrics from the Metrics Server and generates resource recommendations stored in the VerticalPodAutoscaler CRD
2. VPA Updater: Evicts pods that need resource updates based on the recommendations
3. VPA Admission Controller: A mutating webhook that injects recommended resource values into pods during creation

For detailed VPA architecture, see [Kubernetes VPA Documentation](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler).

### Current State

- The central OTLP Gateway DaemonSet has a static resource configuration:
    - `requests.memory`: 32Mi
    - `limits.memory`: 2000Mi
    - Request-to-limit ratio: 62.5x (2000Mi / 32Mi)
- `GOMEMLIMIT` is set dynamically based on the memory limit (80% of limit)
- No automated resource scaling mechanism exists.

## Important Considerations

### Request-to-Limit Ratio

The current request-to-limit ratio of 62.5 is problematic because VPA preserves this ratio when it updates resources. For example, if VPA recommends increasing a Pod's memory request to `64Mi`, it also calculates the new limit as `4000Mi` (62.5 × 64Mi). This calculated limit is likely to exceed the memory capacity of a typical node.
- If VPA recommends `requests.memory` = 64Mi
- VPA will set `limits.memory` = 62.5 × 64Mi = 4000Mi
- This exceeds typical node memory capacity

Before enabling VPA, reduce the request-to-limit ratio to a more reasonable value (for example, 2-4x).

### VPA Limitations

1. Limits Calculation: VPA's `maxAllowed` constraint applies only to requests, not limits. Limits are calculated from the request-to-limit ratio, which can exceed `maxAllowed` values.
2. Scale-Down Timing: VPA makes scale-down decisions based on long-term historical data (typically 8+ days), so resource reductions take time.
3. DaemonSet Updates: VPA requires Pod restarts for resource changes, which means temporary gaps in coverage for DaemonSet Pods. This applies only to clusters that don't support in-place updates (Kubernetes versions before v1.35 or clusters where the feature gate `InPlacePodVerticalScaling` is disabled).

### GOMEMLIMIT Strategy

Because Go-based applications use `GOMEMLIMIT` for soft memory limits, we must decide how to set this value when VPA manages Pod resources:
- Set `GOMEMLIMIT` to a fixed value (for example, 1.6Gi = 80% of 2Gi max). This is recommended for simplicity and predictability.
- Dynamically calculate `GOMEMLIMIT` based on VPA-recommended limits
- Recommendation: Use Option A for simplicity and predictability

## Considered Options

We're evaluating two architectural approaches for implementing VPA with the Central OTLP Gateway. Both options assume a fixed GOMEMLIMIT value (see [GOMEMLIMIT Strategy](#gomemlimit-strategy)).

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
    updateMode: "InPlaceOrRecreate"  # VPA evicts and recreates pods as needed
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
- No Reconciliation Loops: Pod resources are updated with a mutating webhook; the DaemonSet spec remains unchanged, preventing unnecessary reconciliations
- Kubernetes-Native: Uses standard Kubernetes autoscaling components

**Cons:**
- Visibility: DaemonSet spec doesn't reflect actual Pod resources (only visible in Pod specs)
- GOMEMLIMIT Sync: OpenTelemetry Collector's requires extension `cgrouprumtimeextension` to set `GOMEMLIMIT` based on memory limits, which adds complexity

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
3. Update DaemonSet spec when drift exceeds threshold (e.g., >20%)
4. Trigger DaemonSet's built-in rolling update

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

We will go with Option 1: VPA Direct Pod Updates

Rationale:
1. Lower Complexity: Uses tested VPA components rather than implementing custom logic
2. Better Stability: VPA's built-in safeguards (rate limiting, PDB awareness) reduce risk
3. Faster Implementation: Requires only VPA configuration, not code changes
4. Standard Solution: Aligns with Kubernetes best practices and community patterns
5. Acceptable Trade-offs: The visibility limitation is acceptable given monitoring tools can observe actual Pod resources

Configuration Strategy:
- Use `updateMode: "InPlaceOrRecreate"` for automatic Pod updates
- Use `controlledValues: RequestsAndLimits` to avoid ratio-based limit calculations
- Set `GOMEMLIMIT`, [OpenTelemetry Collector provides an extension](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/cgroupruntimeextension) to set `GOMEMLIMIT` in runtime based on available memory, so we can set it to a percentage of the memory limit (e.g., 80%) to ensure it scales with VPA recommendations
- Configure reasonable `minAllowed` and `maxAllowed` boundaries
- Document that actual Pod resources may differ from DaemonSet spec
- The VerticalPodAutoscaler resources will be managed by the telemetry-manager Helm chart.

## Consequences

### Positive Consequences

- Automated Resource Optimization: Pods are automatically sized based on actual usage
- Reduced OOMKills: VPA increases resources before memory pressure occurs
- Cost Efficiency: Resources are reclaimed when usage decreases (with some delay)
- Operational Simplicity: No manual resource tuning required
- Production Proven: VPA is widely used in production Kubernetes clusters

### Negative Consequences

- Monitoring Complexity: We must monitor actual Pod resources, not just the DaemonSet spec
- Documentation Requirement: Need to document that DaemonSet spec doesn't reflect reality
- Coverage Gaps: On clusters without in-place update support, Pod restarts during resource updates can cause brief monitoring gaps
- Scale-Down Delay: Resource reductions take days due to VPA's conservative approach

### Known Limitations

The following limitations are inherent to VPA and should be considered when using it with the Central OTLP Gateway:

1. **Resource Availability**: VPA recommendations may exceed available node resources, causing pods to remain pending. This can be mitigated by using Cluster Autoscaler or configuring appropriate `maxAllowed` limits.
2. **HPA Incompatibility**: VPA cannot be used with Horizontal Pod Autoscaler (HPA) on the same resource metrics (CPU or memory). This is not a concern for DaemonSets, which don't support HPA.
3. **Admission Webhook Conflicts**: VPA's admission controller may conflict with other mutating admission webhooks depending on webhook configuration and ordering.
4. **Out-of-Memory Handling**: While VPA reacts to most OOM events, it cannot handle all out-of-memory scenarios and may not prevent all OOMKills. To scale Pods that are OOMed its important that pod should run for some time so that metrics can be collected.
5. **Multiple VPA Resources**: Configuring multiple VPA resources targeting the same pod results in undefined behavior. Ensure only one VPA resource targets the Central OTLP Gateway DaemonSet.
6. **Recommendations Without Controller**: VPA cannot update resources for standalone pods not managed by a controller (Deployment, DaemonSet, StatefulSet, etc.).

### Some VPA MaxAllowed Strategy

1. Calculate based on node capacity:
   Configure `maxAllowed` values e.g. %75 of smallest node memory capacity to prevent pending pods due to insufficient resources and leave headroom for burstable workloads. E.g., if we have 5 DaemonSets on a 32Gi node: 32Gi × 0.75 ÷ 5 ≈ 4.8Gi each
2. Consider workload patterns:
   Analyze historical usage patterns to set `maxAllowed` values that accommodate typical spikes while preventing excessive resource allocation. The DaemonSets run on every node, one misbehavng DaemonSet can exhaust all nodes.
3. Set based on Application Architecture:
   Consider the current application limits, profile application to find the "knee of the curve" where additional resources provide diminishing returns, and set `maxAllowed` values around that point to optimize cost-performance balance. For example, if the current limit is 2Gi and usage rarely exceeds 1Gi, setting `maxAllowed` to 1.5Gi may be appropriate.
4. Account for QoS and resource ratios:
   If we are using limit = 2 * request, when VPA recomment 10Gi request, it will set 20Gi limit, which is too high. Consider node capacity based on the limit not request.
5. Iteratively adjust:
   Start with conservative `maxAllowed` values and monitor VPA recommendations and pod behavior. Adjust `maxAllowed` as needed based on observed usage patterns and resource availability.