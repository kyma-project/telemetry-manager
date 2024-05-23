# 12. Leader Receiver Creator

Date: 2024-05-21

## Status

Proposed

## Context

As part of metrics collection, the Telemetry module must expose Kubernetes API server metrics. The corresponding OpenTelemetry (OTel) Collector receiver for this task is [Kubernetes Cluster Receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sclusterreceiver). However, there are some open questions:

* How can we ensure that the receiver operates in high availability mode? Should it be integrated into the metric agent (which currently only collects node-affine metrics), incorporated into the gateway (which typically runs multiple replicas), or deployed as a separate component?

* How can we prevent sending duplicate metrics?

## Decision

The easiest way to ensure high availability and prevent duplicate metrics is to use the leader election pattern. In this way, only one replica of OTel Collector will have an active instance of Kubernetes Cluster Receiver running.
The rest of the replicas are in standby mode. If the active instance fails, the standby instance takes over.

The leader election pattern is well known in Kubernetes. The following building blocks are available:
* [Lease](https://kubernetes.io/docs/concepts/architecture/leases/)
* [Resource lock package](https://pkg.go.dev/k8s.io/client-go/tools/leaderelection/resourcelock)
* [Leader election package](https://pkg.go.dev/k8s.io/client-go/tools/leaderelection)

One way to integrate leader election with Kubernetes Cluster Receiver is to bundle the receiver with the leader election logic. However, this approach tightly couples it with the specific receiver. If we need the leader election logic in another context, we will have to reimplement it. Additionally, it will be challenging to contribute this combined implementation to the community, because the receiver already has its own maintainers.

An alternative approach is to create a separate component responsible for leader election that manages another arbitrary sub-receiver. This pattern is already used by the [Receiver Creator](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/receivercreator/README.md), which can instantiate other receivers at runtime based on observed endpoints matching a configured rule.

In this way, the new receiver (let's call it "Leader Receiver Creator") can be configured to create Kubernetes Cluster Receiver only if the underlying instance is a leader. Here's the proposed API:

```yaml
leader_receiver_creator:
  auth_type: serviceAccount
  leader_election:
    lease_name: foo
    lease_namespace: bar
    lease_duration: 15s
    renew_deadline: 10s
    retry_period: 2s
  receiver:
    k8s_cluster:
      auth_type: serviceAccount
      node_conditions_to_report: [Ready, MemoryPressure]
      allocatable_types_to_report: [cpu, memory]
      metrics:
      k8s.container.cpu_limit:
      enabled: false
      resource_attributes:
      container.id:
      enabled: false
```

For a draft implementation, see [github.com/skhalash/leaderreceivercreator](https://github.com/skhalash/leaderreceivercreator).

The new Leader Receiver Creator in combination with Kubernetes Cluster Receiver can be deployed as part of either the metric agent or the metric gateway. However, it is preferable to run it within the metric gateway to avoid an additional network hop, as these metrics are not strictly node-affine.

## Consequences

The new Leader Receiver Creator can be maintained as an internal project and later contributed to the community.
It can be reused in other contexts where leader election is required. It can be bundled into our custom OTel Collector image.

Open questions:

* How to handle multiple different receivers that must follow the leader election pattern?
* How to test leader election?
* How to monitor leader election?

