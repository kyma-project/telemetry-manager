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

#### Advantages

| Category       | Benefit                                                                  |
| -------------- | ------------------------------------------------------------------------ |
| Simplicity     | Minimal new abstractions; builds on existing controller-runtime patterns |
| Migration Path | Incremental refactoring; no new CRDs required                            |
| Latency        | Single reconciliation cycle from pipeline change to workload update      |
| Resource Usage | Fewer etcd objects; lower memory footprint                               |

#### Disadvantages

| Category           | Risk                                                                     |
| ------------------ | ------------------------------------------------------------------------ |
| Coordination       | Multiple controllers may reconcile the same Pipeline CR simultaneously   |
| Status Propagation | Each pipeline must query shared workload status; potential inconsistency |

#### Industry Example

**Prometheus Operator**: Multiple CR types (`ServiceMonitor`, `PodMonitor`, `ProbeMonitor`) are aggregated by a single Prometheus controller into a unified ConfigMap. This demonstrates that multi-watch aggregation is viable when one controller owns the aggregation responsibility.

---

### Option 2: Intermediate Internal CRs

Pipeline controllers create internal CRs (e.g., `GatewayConfig`) that act as aggregation points. A separate deployment controller watches these internal CRs and manages workloads.

![Option 2 Architecture](../assets/031-arch-option2.svg)

#### Advantages

| Category         | Benefit                                                                                               |
| ---------------- | ----------------------------------------------------------------------------------------------------- |
| Separation       | Clear boundary: pipeline controllers handle validation → deployment controller handles infrastructure |
| State Management | Internal CR is single source of truth; aggregation is persisted, not recomputed                       |
| Observability    | `kubectl get` shows aggregated state; provides clear audit trail                                      |
| Testing          | Each layer is testable in isolation                                                                   |

#### Disadvantages

| Category    | Risk                                                                   |
| ----------- | ---------------------------------------------------------------------- |
| Complexity  | Additional CRDs, controllers, and etcd objects to maintain             |
| Latency     | Two reconciliation cycles required (pipeline → internal CR → workload) |
| API Surface | Users may discover and depend on "internal" CRs                        |

#### Industry Example

**Kubernetes Deployment**: The `Deployment → ReplicaSet → Pod` chain demonstrates how intermediate resources provide clear ownership boundaries and enable independent scaling of concerns. Each layer has well-defined responsibilities and can evolve independently.

---

### Option 3: Unified Pipelines Controller

A single "Pipelines Controller" watches all three pipeline CRs, computes aggregated state, and sets computed fields/annotations. Downstream workload controllers react to these enriched pipelines.

![Option 3 Architecture](../assets/031-arch-option3.svg)

#### Advantages

| Category       | Benefit                                                                        |
| -------------- | ------------------------------------------------------------------------------ |
| Coordination   | Single point of aggregation; eliminates race conditions between controllers    |
| Consistency    | Computed state written to pipeline CR itself; downstream reads consistent view |
| Atomic Updates | Can batch changes across all pipeline types before triggering downstream       |

#### Disadvantages

| Category   | Risk                                                                                          |
| ---------- | --------------------------------------------------------------------------------------------- |
| Bottleneck | Single controller becomes critical path; failure affects all pipelines                        |
| Coupling   | Pipelines Controller must understand all pipeline types; grows with new signals               |
| Complexity | "Magic" annotations/fields set by controller are less obvious than explicit CRs or ConfigMaps |

#### Industry Example

**Gateway API**: The `GatewayClass` controller aggregates `Gateway` and `HTTPRoute` configurations into a unified control plane. This pattern works well when strong coordination guarantees are required across related resource types.

---

### Option 4: Intermediate ConfigMaps (Alternative to Option 2)

Similar to Option 2, but instead of introducing new CRD types, pipeline controllers write to an coordinated ConfigMap. A second set of controllers watch this ConfigMap and deploy workloads accordingly.

![Option 4 Architecture](../assets/031-arch-option4.svg)

> [!NOTE]
> Alternatively, to avoid concurrent writes to the same ConfigMap, we can have each pipeline controller write to its own dedicated ConfigMap.
> 
> Each pipeline controller owns then its dedicated ConfigMap:
> - `telemetry-logs-config`
> - `telemetry-metrics-config`
> - `telemetry-traces-config`
>
> The OTLP Gateway controller will then watch all three ConfigMaps and merge them into the final `telemetry-otlp-gateway-config`. Whereas, the Log/Metric Agent controllers will watch logs/metrics ConfigMaps respectively.

#### Advantages

| Category          | Benefit                                                                            |
| ----------------- | ---------------------------------------------------------------------------------- |
| No New APIs       | Avoids additional CRD types and API versioning overhead                            |
| Separation        | Each pipeline controller owns its ConfigMap; no concurrent writes to same resource |
| Kubernetes Native | ConfigMaps are well-understood primitives; no custom resource learning curve       |
| Rollback          | Each intermediate ConfigMap serves as a checkpoint for debugging                   |

#### Disadvantages

| Category      | Risk                                                                             |
| ------------- | -------------------------------------------------------------------------------- |
| Schema        | No schema validation on ConfigMap content; errors caught at runtime              |
| Observability | ConfigMap content is less structured than CR status fields                       |
| Size Limits   | ConfigMaps have a 1MB size limit; may require splitting for large configurations |

#### Industry Example

**Istio Pilot**: Uses ConfigMaps and Secrets as intermediate storage for aggregated xDS configuration before pushing to Envoy sidecars. This demonstrates that ConfigMaps can serve as effective aggregation points without requiring custom CRDs.


### Option 5: Use Pipeline ConfigMaps (Alternative mentioned in Option 4)

This options draw inspiration from alternative approach in Option 4 and use multiple pipeline ConfigMaps to depict different pipelines based on signals.

![Option 5 Architecture](../assets/031-arch-option5.svg)

A typical Pipeline ConfigMap would contain following info
```yaml
LogPipeline:
- name: myNewPipeline1
  generation: 0013
```

`Generation` would be used as it represents the current version of `spec`. When the `spec` for pipeline is updated the generation would also be updated.
The `fluentbit controller` would fetch and read the `fluentbit configmap` thus each controller know exactly which pipelines they must fetch the spec from.

#### Advantages

| Category          | Benefit                                                                            |
| ----------------- | ---------------------------------------------------------------------------------- |
| No New APIs       | Avoids additional CRD types and API versioning overhead                            |
| Separation        | Each pipeline controller owns its ConfigMap; no concurrent writes to same resource |
| Kubernetes Native | ConfigMaps are well-understood primitives; no custom resource learning curve       |

---

## Optimistic Locking on ConfigMaps

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

### References

- [Kubernetes API Conventions: Concurrency Control](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency)
- [controller-runtime: Handling Conflicts](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/client#example-Client-Update)

---

## Comparison Matrix

| Criterion                  | Option 1: Direct Multi-Watch | Option 2: Internal CRs | Option 3: Unified Controller | Option 4: Intermediate ConfigMaps | Option 5: Intermediate Pipeline ConfigMap|
| -------------------------- | ---------------------------- | ---------------------- | ---------------------------- | --------------------------------- |------------------------------------------|
| New CRDs Required          | No                           | Yes                    | No                           | No                                |No                                        |
| Concurrent Write Conflicts | No                           | No                     | No                           | No (if alternative is used)       |No                                        |
| Reconciliation Cycles      | 1                            | 2                      | 1-2                          | 2                                 |2                                         |
| Testability                | Medium                       | High                   | Medium                       | High                              |High                                      |
| Migration Complexity       | Low                          | High                   | Medium                       | Medium                            |Medium                                    |
| Observability              | Low                          | High                   | Medium                       | Medium                            |Medium                                    |

---

## Conclusion

Each option presents valid trade-offs:

- **Option 1** is the simplest but requires coordination for the status propagation.
- **Option 2** provides the cleanest separation but adds API complexity.
- **Option 3** offers strong coordination but creates a bottleneck.
- **Option 4** balances separation and simplicity without new CRDs.
- **Option 5** improves on the idea of `option 4` to provide a simple solution.

The decision should consider:
1. Potential complexity overhead.
2. Simplifying the reconciliation logic, by avoiding unnecessary validations.
3. Observability requirements for debugging pipeline configurations.
4. Future extensibility for additional signal types or changes in the current ones.
5. Alignment with potential Gardener Extension integration.

## Open Questions

- How should potential intermediate ConfigMaps/CRs be structured and what data should they contain?
  > Configmaps would be used with pipelineName and generation.
- How would Gardener Extension integration affect this architecture?
  > When we implement the gardener extension we can revisit this and update the intermediary ConfigMaps with the new schema as per requirement.
- What is the acceptable latency for configuration propagation across the two-cycle options?
  > There should not be latency as the all controllers would run in parallel. When the pipeline ConfigMap is populated with valid pipelines the respective controller would deploy the DaemonSets


## Decision
Architecture diagram from `Option 5` will be implemented.