---
title: Central OTLP Gateway Controller Architecture
status: Proposed
date: 2026-02-06
---

# 31. Central OTLP Gateway Controller Architecture

## Context

Following [ADR-019: Switch from Gateways to a Central Agent](./019-switch-from-gateways-to-a-central-agent.md), we are transitioning from dedicated per-signal gateways to a unified OTLP Gateway running as a DaemonSet. This architectural shift introduces a new challenge: **multiple pipeline controllers (LogPipeline, MetricPipeline, TracePipeline) must now coordinate when reconciling and writing configuration to a shared OTLP Gateway**.

The core problem is concurrent ConfigMap updates. When three independent controllers attempt to write to the same ConfigMap simultaneously:

- Race conditions may cause unnecessary restarts or configuration overwrites
- Status propagation becomes inconsistent across pipeline CRs
- Aggregation logic must be duplicated or awkwardly shared across controllers

This ADR evaluates four architectural approaches to solve the multi-controller coordination problem for the central OTLP Gateway.

![Current Architecture](../assets/031-arch-current.svg)

## Proposal

### Option 1: Direct Multi-Watch

Pipeline controllers manage workloads directly. A new OTLP Gateway controller watches all pipeline CRs and aggregates configurations into a single ConfigMap.

![Option 1 Architecture](../assets/031-arch-option1.svg)

#### Industry Example

**Prometheus Operator**: Multiple CR types (`ServiceMonitor`, `PodMonitor`, `ProbeMonitor`) are aggregated by a single Prometheus controller into a unified ConfigMap. This demonstrates that multi-watch aggregation is viable when one controller owns the aggregation responsibility.

---

### Option 2: Use Pipeline ConfigMaps

Different pipeline controllers write to separate ConfigMaps (e.g., `fluentbit-pipeline-config`, `log-agent-config`, `metric-agent-config`, `otlp-gateway-config`). Respective controllers watch these ConfigMaps for valid pipelines and create DaemonSets accordingly.

We use intermediary ConfigMaps instead of CRs to prevent exposing additional user-facing APIs, reducing potential confusion.
![Option 2 Architecture](../assets/031-arch-option2.svg)

A typical Pipeline ConfigMap would contain following info
```yaml
LogPipeline:
- name: myNewPipeline1
  generation: 0013
```

`Generation` would be used as it represents the current version of `spec`. When the `spec` for pipeline is updated the generation would also be updated.
The `fluentbit controller` would fetch and read the `fluentbit configmap` thus each controller know exactly which pipelines they must fetch the spec from.

The status of pipelines would be handled by two different controllers; e.g. for MetricPipeline `status`, `metric pipeline controller` would be responsible for updating the `config generated` and `flow healthy` conditions. The `Agent Healthy` and `Gateway Healthy` conditions
are handled by `Metric Agent Controller` and `OTLP Gateway Controller` respectively. This way the `Metric Pipeline Controller` would not have knowledge about the workloads thus separating the concerns.

Since multiple controllers would be updating the same ConfigMap `otlp-gateway-config` locking mechanism is required. `controller-runtime` provides out of box `optimistic locking` mechanism which can be used to handle the concurrent updates to the same ConfigMap.

#### Optimistic Locking on ConfigMaps

When multiple controllers write to shared ConfigMaps, Kubernetes provides **optimistic concurrency control** via the `resourceVersion` field. Each ConfigMap has a `resourceVersion` that changes on every update. When a controller attempts to update a ConfigMap:

1. The controller reads the current ConfigMap (including `resourceVersion`)
2. The controller modifies the data and submits an update
3. If the `resourceVersion` in the request matches the current version, the update succeeds
4. If another controller updated the ConfigMap in between, the `resourceVersion` won't match and the update fails with a **409 Conflict**

The controller must then re-read the ConfigMap and retry the update.

```go
// Example: Handling optimistic locking in controller-runtime
err := r.Client.Update(ctx, configMap)
if errors.IsConflict(err) {
    // Re-fetch and retry
    return ctrl.Result{Requeue: true}, nil
}
```

#### References

- [Kubernetes API Conventions: Concurrency Control](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency)
- [controller-runtime: Handling Conflicts](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/client#example-Client-Update)

---

## Decision
Architecture diagram from `Option 2` will be implemented as it divides the responsibilities between different controllers thus simplifies the logic.