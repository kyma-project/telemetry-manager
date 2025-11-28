---
title: Switch from Gateways to a Central Agent
status: Postponed
date: 2025-02-17
---

# 19. Switch from Gateways to a Central Agent

## Context

The current architecture defines a set of central gateways to enrich the data and dispatch it to the backend. That approach had good reasons in the past, but now the drawbacks outweigh the benefits.

![arch](./../assets/otlp-gateway-old.drawio.svg)

### Gateways Benefits

- Persistent buffering via PersistentVolumes (PV)

  The experience showed that persistent buffering on the node filesystem has drawbacks like increased IO and limited space. We always envisioned a configurable persistent size per pipeline, which is only possible in a central StatefulSet having PVs attached. However, experience now shows that operating StatefulSets with PVs is not trivial, and especially on Azure, re-attaching disks can be a very long procedure resulting in long downtimes. Also, persistent buffering might be even a pre-mature optimization with limited gains, see also [ADR-017](./017-fault-tolerant-otel-logging-setup.md).

- Separation of concerns
  
  It is always good to have clear responsibilities to keep code better maintainable. Therefore, gateways deal with enrichment and dispatching, while agents are responsible for data collection.

- Decoupling of signal types

  Processing the signal types in separate processes keeps the processing tolerant to failures. Even when there's an overload of metrics, the log delivery remains stable.

- Tail-based sampling

  Scenarios like trace sampling require processing all spans of a trace at one instance, requiring a StatefulSet with sticky trace routing. However, that scenario might not be relevant as this isn't usually done on the backend.

### Gateways Drawbacks

- No uniform push endpoint

  By design, there is one gateway for each signal type that the application must communicate with. Consequently, you must always configure a separate OTLP endpoint for each signal type. Using a uniform URL requires an additional component to route the requests.

- Autoscaling is tricky

  The gateway should provide autoscaling capabilities, which are tricky to realize. It must be detected very fast if a scale-out is needed, and situations where the backend is overloaded must be excluded from the scale-out.

- Every instance caches the whole cluster state

  Every instance of a gateway must be able to enrich the data of every Pod and with that need to cache all Pod metadata.

- Istio mandatory for load balancing

  Pushing OTLP data from an application to the gateway running with replicas requires load balancing. Unfortunately, a Kubernetes service is acting on L4 while GRPC/HTTP2 is on L7, so an additional load balancer is needed. That can be easily solved by using Istio, which becomes a requirement for applications. However, some applications don't use Istio (StatefulSets often don't) but want to provide OTLP data.

- Istio is mandatory for mTLS

  App-to-gateway communication is across nodes and with that, it should use mTLS; so Istio must be used - which sometimes is impossible. Also, Istio causes additional overhead like noise filtering. Furthermore, collecting observability data about the service mesh should not rely on the service mesh.

- Cumulative to Delta transformation not supported

  The usage of the `cumulativetodelta` processor is mandatory for Dynatrace support and requires that data from a Pod is always processed by the same instance.

- General misconception

  Using simple Kubernetes primitives, it's unrealistic to create a dispatching solution inside a cluster, with a configurable persistent buffer and well-defined guarantees. A queueing system like Kafka will be required and usually will run as a managed solution outside of the cluster. The elements inside the cluster should focus on processing node-local data as fast as possible and delivering the data to the next service, which can provide guarantees.
  Here, we should focus only on the agent scenario: By enriching data node-locally and using the natural scaling done by node instances. In backpressure scenarios, it should not be tried to buffer in a persistent way for a longer time, but instead, propagate the backpressure back to the application so that it can react to it.

## Proposal

Most of the drawbacks can be solved by running the gateway logic node-locally only. Every instance always processes only data of the local node. With that, a natural scalability is given and can be extended by vertical scaling capabilities. The "old" gateways will be running in agent mode but still called "OTLP Gateway". Additionally, there will be a new uniform service `telemetry-otlp` forwarding to the node-local entity using the [internalTrafficPolicy: local](https://kubernetes.io/docs/reference/networking/virtual-ips/#internal-traffic-policy) setting. Existing services can stay compatible using the same approach.

![arch](./../assets/otlp-gateway-new.drawio.svg)

### Remaining Drawbacks of the Agent Approach  

#### Coupling of Signal Types  

Decoupling of signal types will no longer be possible, which may result in scenarios where logs are dropped due to metric overload. However, with proper hardening, this should not pose a significant issue.  

To mitigate the coupling problem, an agent per signal type could be used instead of one shared agent. Let's compare a **shared OTLP agent setup** with an **agent-per-signal setup** in terms of robustness. The OTel Collector offers two key mechanisms to prevent overload:

1. **Memory Limiter**  
   - Applied globally, no way to restrict it to a specific pipeline or signal type.
   - Performs periodic checks of memory usage. When defined limits have been exceeded, it begins refusing data and forcing GC to reduce memory consumption. It is influenced by:
     - Heap-allocated object held by the OTel Collector.  It can be the OTLP exporter sending queues, or various caches used by different components in the chain (like the Kubernetes Attributes Processor informers cache).  
     - Heap-allocated objects that aren't being used, which can be reclaimed by GC.
   - Memory Limiter can not prevent all kinds of OOM issues. It can only prevent issues caused by ingested telemetry data by refusing it. For example, Memory Limiter cannot prevent the issue of the Kubernetes Attributes Processor informer cache consuming excessive memory. Basically, it is a poor man's throttling mechanism that was necessary when using the old Batch Processor. See [ADR-017](./017-fault-tolerant-otel-logging-setup.md).

2. **Refusing Data if OTLP Exporter Queue is Full**  
   - The Batch Processor must not be used (the built-in OTLP Exporter batcher does not have this limitation and can still be used).
   - If the queue is full, new data will be refused.  
   - Each pipeline type has its own queue, allowing different queue lengths to support varying quality-of-service (QoS) levels for different signal types.

Introducing a shared collector handling OTLP data for all three telemetry types means that only one memory limiter is configured, instead of three separate ones when they are isolated. However, if OTLP exporter queue refusal is correctly set up, the system should never reach the point where the memory limiter starts rejecting data. across all clusters is not solvable by either the old or the new proposed setup.

#### Downtime During Updates  

Unlike deployments, the agent cannot be rolled out in a rolling manner, meaning updates will always result in some downtime. However, this should be mitigated by the retry mechanism in the OpenTelemetry SDK (according to our [tests](../../contributor/pocs/otelcol-downtime/otelcol-downtime.md), it's not yet the case).

### Additional Benefits  

#### Persistent Queue Everywhere  

A small persistent buffer can be introduced in all components using the node file system, enhancing resilience and reliability.  

#### No Istio Dependency  

Neither the application nor its components will depend on Istio anymore. However, the metric agent should still support scraping Istio-instrumented endpoints.  


## PoCs

We conducted the following PoCs:
- [OTel Collector Downtime](../../contributor/pocs/otelcol-downtime/otelcol-downtime.md) explores if OTeL SDK performs retries during downtime.
- [Node local traffic](../../contributor/pocs/node-local-traffic/node-local-traffic.md) explores if the node-local traffic is possible with and without Istio.

## Conclusion

The original motivation for the gateway concept is no longer relevant. Transitioning to the agent approach resolves many issues while introducing only minor drawbacks. However, inadequate retry handling by OTel SDKs and Istio proxies remains a challenge. Before proceeding, we must ensure this issue is addressed, considering its alignment with the OTLP specification.

### Agent Rollout and Zero-Downtime Updates

To perform a zero-downtime rollout of a `DaemonSet`, use the `RollingUpdate` update strategy with `maxUnavailable: 0` and `maxSurge: 1`. See the following example:

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: otel
  namespace: otel
spec:
  selector:
    matchLabels:
      app: otel-col
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1
...
```

With this configuration, the `DaemonSet` creates a new Pod on each node before terminating the old one, ensuring that a replacement Pod is fully running before any disruption occurs.

This setup was tested using the OpenTelemetry Demo application to verify that no telemetry data is lost during the rollout. Tests were executed for all three signal types: metrics, traces, and logs. The demo application, which includes services written in various programming languages and uses the OTel SDK, continued sending telemetry to the test collector throughout the rollout without any data loss.

The setup was also tested using `TelemetryGen.` TelemetryGen successfully detected endpoint changes after the rollout and continued sending telemetry to the new endpoints.

The same behavior was confirmed with `Istio access logs`. After the rollout, log data continued flowing to the new endpoints without data loss or duplication.

