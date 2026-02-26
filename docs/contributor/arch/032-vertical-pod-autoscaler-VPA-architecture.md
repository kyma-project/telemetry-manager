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

- Central OTLP Gateway DaemonSet has static resource configuration:
    - `requests.memory`: 32Mi
    - `limits.memory`: 2000Mi
    - Request-to-limit ratio: 62.5x (2000Mi / 32Mi)
- `GOMEMLIMIT` is set dynamically based on the memory limit (80% of limit)
- No automated resource scaling mechanism exists

## Important Considerations

### Request-to-Limit Ratio

The current request-to-limit ratio of 62.5 is problematic for VPA. VPA preserves the ratio when updating resources. For example:
- If VPA recommends `requests.memory` = 64Mi
- VPA will set `limits.memory` = 62.5 × 64Mi = 4000Mi
- This exceeds typical node memory capacity

Reduce the request-to-limit ratio to a more reasonable value (e.g., 2-4x) before enabling VPA.

### VPA Limitations

1. Limits Calculation: VPA's `maxAllowed` constraint applies only to requests, not limits. Limits are calculated from the request-to-limit ratio, which can exceed `maxAllowed` values.
2. Scale-Down Timing: VPA makes scale-down decisions based on long-term historical data (typically 8+ days), so resource reductions take time.
3. DaemonSet Updates: VPA requires pod restarts for resource changes, which means temporary gaps in coverage for DaemonSet pods. This only when the cluster doesn't support in-place updates (The feature gate `InPlacePodVerticalScaling` is enabled by default with Kubernetes version v1.35).

### GOMEMLIMIT Strategy

Since Go-based applications use `GOMEMLIMIT` for soft memory limits:
- Option A: Set `GOMEMLIMIT` to a fixed value (e.g., 1.6Gi = 80% of 2Gi max)
- Option B: Dynamically calculate `GOMEMLIMIT` based on VPA-recommended limits
- Recommendation: Use Option A for simplicity and predictability

## Considered Options

### Option 1: VPA Direct Pod Updates (Recommended)

Allow VPA Updater to directly manage pod resources through the VPA Admission Controller.

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
- Stability: VPA considers Priority Class, Pod Disruption Budget, and eviction rate limits when updating pods
- Proven Solution: Complex decision logic (when to evict, which pods to evict) is handled by well-tested VPA components
- No Reconciliation Loops: Pod resources are updated via mutating webhook; DaemonSet spec remains unchanged, preventing unnecessary reconciliations
- Kubernetes-Native: Leverages standard Kubernetes autoscaling components

**Cons:**
- Visibility: DaemonSet spec doesn't reflect actual pod resources (only visible in pod specs)
- GOMEMLIMIT Sync: OpenTelemetry Collector's requires extension `cgrouprumtimeextension` to set `GOMEMLIMIT` based on memory limits, which adds complexity

### Option 2: Reconciler-Driven Updates

Implement custom logic in the telemetry-manager reconciler to watch VPA recommendations and update the DaemonSet spec accordingly.

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

The reconciler would:
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
- Reinventing the Wheel: Duplicates functionality that VPA already provides

## Decision

We will go with Option 1: VPA Direct Pod Updates

Rationale:
1. Lower Complexity: Leverages tested VPA components rather than implementing custom logic
2. Better Stability: VPA's built-in safeguards (rate limiting, PDB awareness) reduce risk
3. Faster Implementation: Requires only VPA configuration, not code changes
4. Standard Solution: Aligns with Kubernetes best practices and community patterns
5. Acceptable Trade-offs: The visibility limitation is acceptable given monitoring tools can observe actual pod resources

Configuration Strategy:
- Use `updateMode: "InPlaceOrRecreate"` for automatic pod updates
- Use `controlledValues: RequestsAndLimits` to avoid ratio-based limit calculations
- Set `GOMEMLIMIT`, [OpenTelemetry Collector provides an extension](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/cgroupruntimeextension) to set `GOMEMLIMIT` in runtime based on available memory, so we can set it to a percentage of the memory limit (e.g., 80%) to ensure it scales with VPA recommendations
- Configure reasonable `minAllowed` and `maxAllowed` boundaries
- Document that actual pod resources may differ from DaemonSet spec
- The VerticalPodAutoscaler resources will be managed by the telemetry-manager Helm chart.

## Consequences

### Positive Consequences

- Automated Resource Optimization: Pods automatically sized based on actual usage
- Reduced OOMKills: VPA increases resources before memory pressure occurs
- Cost Efficiency: Resources reclaimed when usage decreases (with some delay)
- Operational Simplicity: No manual resource tuning required
- Production Proven: VPA is widely used in production Kubernetes clusters

### Negative Consequences

- Monitoring Complexity: Must monitor actual pod resources, not just DaemonSet spec
- Documentation Requirement: Need to document that DaemonSet spec doesn't reflect reality
- Coverage Gaps: DaemonSet pods restart during resource updates (brief monitoring gaps), when cluster doesn't support in-place updates.
- Scale-Down Delay: Resource reductions take days due to VPA's conservative approach

### Known Limitations

The following limitations are inherent to VPA and should be considered when using it with the Central OTLP Gateway:

1. **Resource Availability**: VPA recommendations may exceed available node resources, causing pods to remain pending. This can be mitigated by using Cluster Autoscaler or configuring appropriate `maxAllowed` limits.
2. **HPA Incompatibility**: VPA cannot be used with Horizontal Pod Autoscaler (HPA) on the same resource metrics (CPU or memory). This is not a concern for DaemonSets, which don't support HPA.
3. **Admission Webhook Conflicts**: VPA's admission controller may conflict with other mutating admission webhooks depending on webhook configuration and ordering.
4. **Out-of-Memory Handling**: While VPA reacts to most OOM events, it cannot handle all out-of-memory scenarios and may not prevent all OOMKills.
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