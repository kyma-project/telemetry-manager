---
title: VPA for Self-Monitor Prometheus
status: Proposed
date: 2026-06-23
---

# VPA for Self-Monitor Prometheus

## Context and Problem Statement

The self-monitor is a single-replica Prometheus Deployment that evaluates alert rules every 30 seconds and drives the `FlowHealthy` status condition on all pipeline CRDs. It has a 2-hour retention window and a 50 MB size cap, and it scrapes only the telemetry agents and gateways within the `kyma-system` namespace.

The current static resource allocation (`memoryRequest: 50Mi`, `memoryLimit: 512Mi`) causes two problems: the request is far below steady-state usage, leading to poor scheduling bin-packing, and the static limits either waste reserved capacity (when set high) or cause OOMKill loops (when set low for workloads with startup spikes).

VPA support was added in [#3511](https://github.com/kyma-project/telemetry-manager/issues/3511) and reverted in [#3579](https://github.com/kyma-project/telemetry-manager/issues/3579) because the pod was OOMKilled on every startup. The root cause was using `controlledValues: RequestsAndLimits` — VPA would reduce both requests and limits during sustained low-memory periods, and when the pod restarted, the WAL replay spike exceeded the reduced limit. This ADR proposes a VPA configuration that prevents VPA from shrinking the memory headroom needed for WAL replay.

See [#3615](https://github.com/kyma-project/telemetry-manager/issues/3615) for more information.

### Background: Prometheus WAL Startup Spike

When Prometheus starts, it replays its Write-Ahead Log (WAL) to restore in-memory state. The WAL holds all samples from the last two hours (controlled by `--storage.tsdb.min-block-duration`). During replay, Prometheus allocates scratch memory proportional to the WAL size, producing a peak that is typically more than the steady-state working set.

For the self-monitor, steady-state memory is low because it scrapes only a handful of targets with short retention. However, after any pod restart, the WAL replay spike still occurs. If VPA has reduced the limit below that spike, the pod is OOMKilled before it is ever ready — and the pod restarts again, in an OOMKill loop.

Because the self-monitor uses an `EmptyDir` volume for storage, every unscheduled restart (eviction, node drain, or OOMKill) discards the existing TSDB blocks and WAL. On the next start, Prometheus rebuilds the in-memory state by scraping fresh data, not by replaying a WAL (because the WAL was also discarded). However, the steady-state working set is still determined by the 2-hour retention window and the number of active time series.

### Memory Footprint Drivers

Self-monitor memory consumption is dominated by three factors:

- **Number of nodes**: each node contributes a set of active time series for the four OTel Collector metrics (`exporter_sent`, `exporter_send_failed`, `exporter_enqueue_failed`, `receiver_refused`) across log, metric, and trace pipelines.
- **Number of pipelines per signal type**: each pipeline adds another set of time series per node.
- **WAL replay on startup**: scratch memory during replay is 2–3x steady-state.

Steady-state memory is dominated by the time-series cardinality, while the startup spike is dominated by WAL replay. Both grow with cluster size and pipeline count, but slowly: with the current set of scraped metrics, even multi-hundred-node clusters fit within a few hundred MiB at steady state.

### How Other Projects Handle This

Gardener's Prometheus VPA configuration ([source](https://github.com/gardener/gardener/blob/master/pkg/component/observability/monitoring/prometheus/vpa.go)) uses `controlledValues: RequestsOnly` with `minAllowed.memory: 150Mi` and no static memory limit on the container. VPA manages only the memory request for scheduling efficiency, and Prometheus is free to allocate as much memory as the node has available — bounded only by VPA's `maxAllowed` recommendation cap.

The Prometheus Operator community follows the same pattern. The reasoning: static limits on a workload with a large startup spike either waste reserved capacity (if set high enough to cover the spike) or cause OOMKill loops (if set lower). Removing the limit lets the kernel absorb transient spikes while the scheduler still receives a meaningful request through VPA.

## Considered Options

### Option 1: VPA with RequestsOnly, No Container Memory Limit (Recommended)

Re-enable VPA for the self-monitor with `controlledValues: RequestsOnly`, no static memory limit on the container, and a startup-friendly Prometheus configuration. This follows the Gardener and Prometheus Operator pattern.

**VPA Configuration:**

```yaml
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: telemetry-self-monitor
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: telemetry-self-monitor
  updatePolicy:
    updateMode: InPlaceOrRecreate
  resourcePolicy:
    containerPolicies:
    - containerName: self-monitor
      controlledResources: ["memory"]
      controlledValues: RequestsOnly
      minAllowed:
        memory: 128Mi
      maxAllowed:
        memory: 512Mi
```

The cap of 512 MiB matches the current static memory limit for the self-monitor pod, ensuring VPA does not recommend requests higher than the previous ceiling.

**Memory Limit:**

The container memory limit is removed from the pod spec. Prometheus can allocate memory as needed, with no `GOMEMLIMIT` set on the process. The Linux kernel enforces the node-level memory ceiling, so a runaway workload cannot exhaust the node.

This is a deliberate departure from the OTel-component pattern in [ADR 033](033-vertical-pod-autoscaler-VPA-architecture.md), which uses `RequestsAndLimits`. The reasons are specific to Prometheus:

- WAL replay creates a 2–3x startup spike that has no equivalent in OTel Collectors. Sizing a limit that covers the spike means wasting that reservation in steady state.
- VPA cannot safely reduce the limit on a workload with a startup spike — the original failure mode of [#3511](https://github.com/kyma-project/telemetry-manager/issues/3511).
- The self-monitor is a single replica in `kyma-system`; the blast radius of unbounded memory is contained.

**Memory Request:**

The initial static memory request is raised from `50Mi` to `128Mi` to match the VPA `minAllowed` floor. This anchors the first scheduling cycle before VPA has gathered enough data to make recommendations.

**Startup Probe:**

The current liveness probe starts immediately (5s × 5 failures = 25s grace), which is not enough for Prometheus to initialize on busy clusters. Without protection, the pod can be killed before reaching readiness. Because the self-monitor uses an `EmptyDir` volume, a restart discards the WAL entirely, and the next start begins with a fresh, empty TSDB. The steady-state memory footprint still depends on the 2-hour retention window and the number of active time series.

Add a startup probe so the liveness probe only begins after Prometheus signals readiness:

```go
startup := &corev1.Probe{
    ProbeHandler: corev1.ProbeHandler{
        HTTPGet: &corev1.HTTPGetAction{
            Path: "/-/ready",
            Port: intstr.IntOrString{IntVal: selfmonports.PrometheusPort},
        },
    },
    InitialDelaySeconds: 10,
    PeriodSeconds:       10,
    FailureThreshold:    30, // 5 minutes total budget for TSDB initialization
    TimeoutSeconds:      5,
    SuccessThreshold:    1,
}
```

A `WithStartupProbe` helper must be added to `internal/resources/common/pod_spec.go` alongside the existing `WithProbes`, then wired into the self-monitor container builder.

**Pros:**
- VPA right-sizes the memory request based on observed usage, improving scheduling efficiency
- The pod cannot be OOMKilled by VPA shrinking its memory limit (because there is no container memory limit)
- The startup probe gives Prometheus up to 5 minutes to reach readiness
- The configuration is minimal: no workload-derived formulas, no node-count tracking, no reconciler logic tied to cluster topology for sizing

**Cons:**
- The self-monitor uses a different VPA configuration (`RequestsOnly` with no container limit) than other telemetry components (`RequestsAndLimits` per [ADR 033](033-vertical-pod-autoscaler-VPA-architecture.md)). This is intentional and tied to the Prometheus WAL replay characteristic; the exception must be documented.
- Without a container memory limit, an unexpected memory regression in Prometheus (or a future increase in scrape volume) is bounded only by node-level memory pressure rather than by a pod-level limit.

### Option 2: VPA with RequestsAndLimits

Enable VPA with `controlledValues: RequestsAndLimits`, allowing VPA to manage both memory requests and limits.

**Pros:**
- Standard VPA configuration, as used for OTel components (ADR 033)

**Cons:**
- This is the failure mode of the original implementation ([#3511](https://github.com/kyma-project/telemetry-manager/issues/3511)): VPA reduces limits during low-memory periods, then the pod restarts and the WAL replay spike exceeds the reduced limit, causing an OOMKill loop

## Decision

Adopt **Option 1: VPA with RequestsOnly, No Container Memory Limit**.

VPA will right-size requests for scheduling efficiency, and we prevent VPA from reducing limits below the WAL replay peak by removing the container memory limit entirely. This follows the pattern used by Gardener and the Prometheus Operator community.

The decision deliberately keeps the configuration simple. We do not introduce a workload-derived memory limit at this point: the current set of scraped metrics is small enough that production memory is expected to stay well below typical node capacity, and the team has chosen to observe real production behavior before adding any limit.


## Consequences

### Positive Consequences

- The self-monitor pod can no longer be OOMKilled by VPA shrinking its memory limit, because there is no container memory limit.
- VPA right-sizes the memory request based on observed usage, improving scheduling on clusters where the self-monitor is under-utilized.
- The startup probe gives Prometheus up to 5 minutes to reach readiness, eliminating the failure mode where the liveness probe killed the pod before initialization completed.
- The configuration is minimal: no workload-derived formulas, no node-count tracking, no reconciler logic tied to cluster topology for sizing.

### Negative Consequences

- The self-monitor uses a different VPA configuration (`RequestsOnly` with no container limit) than other telemetry components (`RequestsAndLimits` per [ADR 033](033-vertical-pod-autoscaler-VPA-architecture.md)). This is intentional and tied to the Prometheus WAL replay characteristic; the exception must be documented.

### Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Without a container memory limit, an unexpected memory regression in Prometheus or a future increase in scrape volume is bounded only by node-level memory pressure, not a pod-level limit. | Monitor the self-monitor's actual memory consumption in production. If footprint grows materially (for example, after [#2955](https://github.com/kyma-project/telemetry-manager/issues/2955) lands), revisit the limit decision and introduce a percentage-of-node-size limit using the existing `nodesize.VPAMaxAllowedMemory()` helper (reusing the pattern from [ADR 033](033-vertical-pod-autoscaler-VPA-architecture.md)). The exact percentage should be chosen based on observed memory in production. |
| The startup probe allows up to 5 minutes for Prometheus to reach readiness. On clusters with very high cardinality, this may not be enough. | If startup probe failures are observed in production, increase `failureThreshold` to extend the startup budget, or investigate why the TSDB initialization is taking longer than expected (for example, disk I/O bottleneck, unexpected scrape volume). |

