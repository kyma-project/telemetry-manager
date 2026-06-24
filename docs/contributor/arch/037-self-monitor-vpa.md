---
title: VPA for Self-Monitor Prometheus
status: Proposed
date: 2026-06-23
---

# VPA for Self-Monitor Prometheus

## Context and Problem Statement

The self-monitor is a single-replica Prometheus Deployment that evaluates alert rules every 30 seconds and drives the `FlowHealthy` status condition on all pipeline CRDs. It has a 2-hour retention window and a 50 MB size cap, and it scrapes only the telemetry agents and gateways within the `kyma-system` namespace.

Its current static resource allocation (`memoryRequest: 50Mi`, `memoryLimit: 512Mi`) is too coarse: the request is far below steady-state usage, causing poor scheduling bin-packing and wasted reserved capacity.

VPA support was added in [#3511](https://github.com/kyma-project/telemetry-manager/issues/3511) and reverted in [#3579](https://github.com/kyma-project/telemetry-manager/issues/3579) because the pod was OOMKilled on every startup. The root cause was using `controlledValues: RequestsAndLimits` — VPA observed low steady-state memory, shrank both request and limit to match, and the next restart killed the pod before it became ready. This ADR investigates the failure mode and proposes the correct VPA configuration.

See [#3615](https://github.com/kyma-project/telemetry-manager/issues/3615) for more information.

### Background: Prometheus WAL Startup Spike

When Prometheus starts, it replays its Write-Ahead Log (WAL) to restore in-memory state. The WAL holds all samples from the last two hours (controlled by `--storage.tsdb.min-block-duration`). During replay, Prometheus allocates scratch memory proportional to the WAL size, producing a peak that is typically 2–3x the steady-state working set.

For the self-monitor, steady-state memory is low because it scrapes only a handful of targets with short retention. However, after any pod restart, the WAL replay spike still occurs. If VPA has reduced the limit below that spike, the pod is OOMKilled before it is ever ready — and the pod restarts again, in an OOMKill loop.

Because the self-monitor uses an `EmptyDir` volume for storage, every unscheduled restart (eviction, node drain, or OOMKill) discards the existing TSDB blocks. On the next start, Prometheus rebuilds from scratch, which may produce an even larger WAL depending on how much data had not yet been compacted to blocks.

### Memory Footprint Drivers

Self-monitor memory consumption is dominated by three factors:

- **Number of nodes**: each node contributes a set of active time series for the four OTel Collector metrics (`exporter_sent`, `exporter_send_failed`, `exporter_enqueue_failed`, `receiver_refused`) across log, metric, and trace pipelines.
- **Number of pipelines per signal type**: each pipeline adds another set of time series per node.
- **WAL replay on startup**: scratch memory during replay is 2–3x steady-state.

Steady-state memory is dominated by the time-series cardinality, while the startup spike is dominated by WAL replay. Both grow with cluster size and pipeline count, but slowly: with the current set of scraped metrics, even multi-hundred-node clusters fit within a few hundred MiB at steady state.

### How Other Projects Handle This

Gardener's Prometheus VPA configuration (`pkg/component/observability/monitoring/prometheus/vpa.go`) uses `controlledValues: RequestsOnly` with `minAllowed.memory: 150Mi` and **no static memory limit** on the container. VPA manages only the memory request for scheduling efficiency, and Prometheus is free to allocate as much memory as the node has available — bounded only by VPA's `maxAllowed` recommendation cap.

The Prometheus Operator community follows the same pattern. The reasoning: static limits on a workload with a large startup spike either waste reserved capacity (if set high enough to cover the spike) or cause OOMKill loops (if set lower). Removing the limit lets the kernel absorb transient spikes while the scheduler still receives a meaningful request through VPA.

## Decision

Re-enable VPA for the self-monitor with `controlledValues: RequestsOnly`, no static memory limit on the container, and a startup-friendly Prometheus configuration. This follows the Gardener and Prometheus Operator pattern referenced above.

The decision deliberately keeps the configuration simple. We do not introduce a workload-derived memory limit at this point: the current set of scraped metrics is small enough that production memory is expected to stay well below typical node capacity, and the team has chosen to observe real production behavior before adding any limit. If the self-monitor's footprint grows in a future iteration (see [Future Considerations](#future-considerations)), a simple percentage-of-node-size limit can be added later.

Other approaches (no VPA at all, or VPA with `RequestsAndLimits`) are not considered viable: we want VPA to right-size requests for scheduling efficiency, and we must stop letting VPA reduce limits below the WAL replay peak.

### VPA Configuration

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
        memory: <see formula below>
```

VPA manages only the memory request. The `maxAllowed` is computed from the observed cluster node count and caps how high VPA can push the request, so VPA cannot recommend a request that the smallest node could not honor.

```
maxAllowedMemory = min(32Mi + nodeCount × 16Mi, 512Mi)
```

Worked examples:

| Nodes | Formula result | `maxAllowed` |
|------:|---------------:|-------------:|
| 1     | 48 MiB         | 48 MiB       |
| 10    | 192 MiB        | 192 MiB      |
| 30    | 512 MiB        | 512 MiB      |
| 100   | 1 632 MiB      | 512 MiB      |

The cap of 512 MiB keeps VPA's request recommendation modest in absolute terms. The base of 32 MiB ensures small clusters still get a usable ceiling that VPA can grow into.

### Memory Limit

The container memory limit is removed from the pod spec. Prometheus is free to allocate memory as needed, with no `GOMEMLIMIT` set on the process. The Linux kernel still enforces the node-level memory ceiling, so a runaway workload cannot exhaust the node.

This is a deliberate departure from the OTel-component pattern in [ADR 033](033-vertical-pod-autoscaler-VPA-architecture.md), which uses `RequestsAndLimits`. The reasons are specific to Prometheus:

- WAL replay creates a 2–3x startup spike that has no equivalent in OTel Collectors. Sizing a limit that covers the spike means wasting that reservation in steady state.
- VPA cannot safely reduce the limit on a workload with a startup spike — the original failure mode of [#3511](https://github.com/kyma-project/telemetry-manager/issues/3511).
- The self-monitor is a single replica in `kyma-system`; the blast radius of unbounded memory is contained.

### Memory Request

The initial static memory request is raised from `50Mi` to `128Mi` to match the VPA `minAllowed` floor. This anchors the first scheduling cycle before VPA has gathered enough data to make recommendations.

### Startup Probe

The current liveness probe starts immediately (5s × 5 failures = 25s grace), which is not enough for WAL replay on busy clusters. Without protection, the pod can be killed mid-replay, looping into another replay attempt that allocates even more memory.

Add a startup probe so the liveness probe only takes over after Prometheus has successfully completed replay:

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
    FailureThreshold:    30, // 5 minutes total budget for WAL replay
    TimeoutSeconds:      5,
    SuccessThreshold:    1,
}
```

A `WithStartupProbe` helper must be added to `internal/resources/common/pod_spec.go` alongside the existing `WithProbes`, then wired into the self-monitor container builder.

### Prometheus Version and WAL Flags

The self-monitor ships Prometheus **3.11.3** (built from source in `dependencies/telemetry-self-monitor/Dockerfile`, FIPS variant is `prometheus-fips:3.11.3`).

Two WAL-related flags are evaluated here:

- **`--storage.tsdb.wal-compression`**: enabled by default since Prometheus 2.20. On 3.11.3 it is already on; passing the flag explicitly is a no-op. **No change required.**
- **`--enable-feature=memory-snapshot-on-shutdown`**: writes an in-memory snapshot to `--storage.tsdb.path` at graceful shutdown so the next start can skip WAL replay. This flag provides no benefit with the self-monitor's `EmptyDir` storage. When a pod is deleted or evicted — by VPA in `Recreate` mode, by a node drain, or after an OOMKill loop — the `EmptyDir` volume is destroyed along with the pod and the snapshot is gone before the replacement pod starts. The only scenario where it could help is a container restart within the same running pod, but OOMKill delivers `SIGKILL` with no opportunity for a graceful write. **Not adopted.** The flag would only become useful if storage were migrated to a PersistentVolume, which this ADR explicitly rejects.

No new flags are added to the argument list.

### Rejected Alternatives

The following ideas were discussed and rejected for this iteration:

- **Persistent Volume instead of `EmptyDir`**: would let Prometheus survive restarts without rebuilding from scratch, but adds a PVC dependency, complicates self-monitor lifecycle (provisioning, deletion, finalization), and is over-engineering for a 2-hour-retention monitoring instance. Keep `EmptyDir`.
- **Static memory limit without VPA**: static-only sizing wastes reservation on small clusters and under-provisions on large ones. We want VPA to right-size the request.
- **VPA with `RequestsAndLimits`**: this is the failure mode of the original implementation. VPA scales the limit proportionally to the request, and any sustained low-memory period pushes the limit below the WAL replay peak.
- **Workload-derived static limit** (per-node series count × cluster size, capped at an absolute ceiling): adds complexity for no clear benefit at the current scope. The team prefers to observe production memory under the simple `RequestsOnly` setup before deciding whether any limit is needed.

## Future Considerations

### Scraping Metric Agent Targets

Issue [#2955](https://github.com/kyma-project/telemetry-manager/issues/2955) tracks expanding the self-monitor to scrape metric agent targets for operations monitoring. Doing so increases the time-series cardinality the self-monitor stores, which raises both steady-state memory and the WAL replay spike.

When that work lands, a container memory limit may become advisable to bound the worst-case footprint. The recommended approach is a simple percentage of the smallest node's allocatable memory (reusing the existing `nodesize.VPAMaxAllowedMemory()` helper used for OTel components in [ADR 033](033-vertical-pod-autoscaler-VPA-architecture.md)), not a workload-derived formula. The exact percentage should be chosen based on observed memory in production after [#2955](https://github.com/kyma-project/telemetry-manager/issues/2955) is rolled out.

### Sharding

If a single self-monitor instance ever outgrows a node — well beyond what is foreseeable today — the standard Prometheus pattern is to shard by scrape target using `hashmod` relabeling. The scrape configuration is already generated programmatically (`internal/selfmonitor/config/config_builder.go`), and the alert webhook handler already de-duplicates by pipeline name, so the change would be localized. This is not on the near-term roadmap.

## Consequences

### Positive Consequences

- The self-monitor pod can no longer be OOMKilled by VPA shrinking its memory limit, because there is no container memory limit.
- VPA right-sizes the memory request based on observed usage, improving scheduling on clusters where the self-monitor is under-utilized.
- The startup probe gives WAL replay up to 5 minutes to complete, eliminating the failure mode where the liveness probe killed the pod mid-replay.
- The configuration is minimal: no workload-derived formulas, no node-count tracking, no reconciler logic tied to cluster topology for sizing.

### Negative Consequences

- The self-monitor uses a different VPA configuration (`RequestsOnly` with no container limit) than other telemetry components (`RequestsAndLimits` per [ADR 033](033-vertical-pod-autoscaler-VPA-architecture.md)). This is intentional and tied to the Prometheus WAL replay characteristic; the exception must be documented.
- Without a container memory limit, an unexpected memory regression in Prometheus (or a future increase in scrape volume) is bounded only by node-level memory pressure rather than by a pod-level limit. The mitigation is operational: monitor the self-monitor's actual memory in production and revisit the limit decision if footprint grows materially (see [Future Considerations](#future-considerations)).

## Test Plan

The test plan covers manual cluster verification, a restart stress test that reproduces the original failure mode, and a high-cardinality simulation for large-cluster confidence. Unit tests for the resource builder and reconciler integration tests are implicit deliverables of the implementation PR and are not detailed here.

### Manual Cluster Verification

```bash
# Provision a cluster and deploy
make provision-k3d && make deploy-experimental

# Confirm the VPA resource has the correct shape
kubectl -n kyma-system get vpa telemetry-self-monitor -o yaml
#   spec.resourcePolicy.containerPolicies[0].controlledValues: RequestsOnly
#   spec.resourcePolicy.containerPolicies[0].minAllowed.memory: 128Mi
#   spec.resourcePolicy.containerPolicies[0].maxAllowed.memory: min(32Mi + nodeCount × 16Mi, 512Mi)

# Wait ~5 minutes for VPA to gather metrics, then inspect the recommendation
kubectl -n kyma-system get vpa telemetry-self-monitor \
  -o jsonpath='{.status.recommendation.containerRecommendations[0]}'

# Confirm the pod has no memory limit and the request matches VPA's recommendation
kubectl -n kyma-system get pod -l app.kubernetes.io/name=telemetry-self-monitor \
  -o jsonpath='{.items[0].spec.containers[0].resources}' | jq
#   limits.memory must be absent
#   requests.memory must reflect VPA's recommendation (after the first VPA cycle)
```

### Restart Stress Test (Reproduces the Original Failure Mode)

This test reproduces the OOMKill loop that caused the original VPA revert. It requires VPA to be present and to have gathered at least 30 minutes of data, so its recommendation is at or below the observed steady-state.

```bash
# 1. Run for at least 30 minutes so VPA establishes a steady-state recommendation.
# 2. Snapshot the VPA recommendation.
kubectl -n kyma-system get vpa telemetry-self-monitor \
  -o jsonpath='{.status.recommendation}' > /tmp/vpa-snapshot.json

# 3. Force three hard restarts in succession (simulates OOMKill or node eviction).
for i in 1 2 3; do
  kubectl -n kyma-system delete pod -l app.kubernetes.io/name=telemetry-self-monitor --force --grace-period=0
  kubectl -n kyma-system wait pod -l app.kubernetes.io/name=telemetry-self-monitor --for=condition=Ready --timeout=300s
done

# 4. Verify no OOMKill events were emitted during the three restarts.
kubectl -n kyma-system get events \
  --field-selector reason=OOMKilling,involvedObject.kind=Pod \
  -o jsonpath='{range .items[*]}{.involvedObject.name}{"\n"}{end}' \
  | grep telemetry-self-monitor && echo "FAIL: OOMKilled" || echo "PASS"

# 5. Verify the startup probe absorbed WAL replay (look for the startup probe result line).
kubectl -n kyma-system describe pod -l app.kubernetes.io/name=telemetry-self-monitor \
  | grep -A3 "Startup:"
```

**Pass criteria:** All three pods reach `Ready` within 5 minutes, no OOMKill events, and the startup probe shows at least one successful check before the liveness probe took over.

### High-Cardinality Scale Simulation

The goal is to inflate the scrape target count on a small test cluster to a level equivalent to a 200+ node production cluster, then observe the self-monitor's memory behavior under load and after a forced restart. The output is a record of actual memory consumption that informs future decisions about whether a memory limit is needed.

**Why not just use a large cluster:** The self-monitor's memory scales with the number of active time series, not the number of physical nodes per se. Each OTel Collector pod (agent or gateway) exposes the four key series per pipeline. By deploying many synthetic collector-like pods on a small cluster, each advertising the expected metric labels, we can reproduce the time-series volume of a large cluster without provisioning one.

**Setup:**

1. On a 3-node k3d cluster, create the maximum expected pipeline set: 5 `LogPipeline`, 5 `MetricPipeline`, and 5 `TracePipeline` resources, all pointing at a reachable (but otherwise unused) OTLP endpoint.

2. Deploy a fleet of synthetic scrape targets. These are minimal HTTP servers that expose a static Prometheus metrics page containing the four self-monitor series (`otelcol_exporter_sent_metric_points_total`, `otelcol_exporter_send_failed_metric_points_total`, `otelcol_exporter_enqueue_failed_metric_points_total`, `otelcol_receiver_refused_metric_points_total`) with labels matching the self-monitor's scrape config (for example, `job="telemetry-metric-gateway"`, `kyma_pipeline_name="pipeline-N"`). Use a Deployment with 200 replicas to simulate 200 nodes' worth of collectors.

   A minimal synthetic exporter can be a ConfigMap-mounted Nginx config serving static metric text on port 8888:

   ```yaml
   # static-metrics.txt (one set per pipeline per collector pod)
   otelcol_exporter_sent_metric_points_total{pipeline="metric-pipeline-0"} 1000
   otelcol_exporter_send_failed_metric_points_total{pipeline="metric-pipeline-0"} 0
   otelcol_exporter_enqueue_failed_metric_points_total{pipeline="metric-pipeline-0"} 0
   otelcol_receiver_refused_metric_points_total{pipeline="metric-pipeline-0"} 0
   # ... repeat for all 5 pipelines
   ```

3. Patch the self-monitor's scrape config (or deploy a `ServiceMonitor`-equivalent) to scrape all 200 replicas. Alternatively, annotate the synthetic pods with `telemetry.kyma-project.io/scrape: "true"` if the self-monitor's scrape config already uses that selector.

**Observation window:**

- Wait 15 minutes for the self-monitor to scrape all targets at least once and for WAL to accumulate.
- Record steady-state memory: `kubectl top pod -n kyma-system -l app.kubernetes.io/name=telemetry-self-monitor`.
- Force a hard pod delete and measure peak memory during WAL replay: poll `kubectl top` every 5 seconds for the first 2 minutes after the replacement pod starts.
- Record the VPA recommendation reached after the run: `kubectl -n kyma-system get vpa telemetry-self-monitor -o jsonpath='{.status.recommendation}'`.

**Pass criteria:**

- The pod must reach `Ready` within the 5-minute startup probe budget without OOMKill across the steady-state run and the restart.
- The observed peak memory and VPA recommendation must be recorded for use as the production-sizing baseline. There is no upper-bound assertion at this stage — the purpose of the run is to characterize behavior, not to validate against a formula.
