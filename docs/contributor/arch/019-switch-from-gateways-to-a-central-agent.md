# 19. Switch from Gateways to a Central Agent

Date: 2025-02-17

## Status

Proposed

## Context

The current architecture defines a set of central Gateways to enrich the data and dispatch it to the backend. That approach had good reasons in the past, but now the drawbacks outweigh the benefits.

![arch](./../assets/otlp-gateway-old.drawio.svg)

### Gateways Benefits

- Persistent buffering via PV

  The experience showed that persistent buffering on the node filesystem has drawbacks like increased IO and limited space. We always envisioned a configurable persistent size per pipeline which is only possible in a central StatefulSet having PVs attached. However, experience now shows that operating StatefulSets with PVs is not trivial, and especially on Azure, re-attaching disks can be a very long procedure resulting in long downtimes. Also, persistent buffering might be even a pre-mature optimization with limited gains, see also [ADR-017](./017-fault-tolerant-otel-logging-setup.md).

- Separation of concerns
  
  It is always good to have clear responsibilities to keep code better maintainable. Therefore, gateways deal with enrichment and dispatching, while agents are responsible for data collection.

- Decoupling of signal types

  Processing the signal types in separate processes keeps the processing tolerant to failures. An overload of metrics keeps the log delivery stable.

- Tail-based sampling

  Scenarios like trace sampling require processing all spans of a trace at one instance, requiring a StatefulSet with sticky trace routing. However, that scenario might not be relevant as this isn't usually done on the backend.

### Gateways Drawbacks

- No uniform push endpoint

  By design, there is one gateway for each signal type that the application must communicate with. Consequently, you must always configure a separate OTLP endpoint for each signal type. Using a uniform URL requires an additional component to route the requests.

- Autoscaling is tricky

  The gateway should provide autoscaling capabilities which are tricky to realize. It must be detected very fast if a scale-out is needed and situations where the backend is overloaded must be excluded from the scale-out.

- Every instance caches the whole cluster state

  Every instance of a gateway must be able to enrich the data of every Pod and with that need to cache all Pod metadata.

- Istio mandatory for load balancing

  Pushing OTLP data from an application to the gateway running with replicas requires load balancing. Unfortunately, a Kubernetes service is acting on L4 while GRPC/HTTP2 is on L7, so an additional load balancer is needed. That can be easily solved by using Istio, which becomes a requirement for applications. However, some applications don't use Istio (StatefulSets often don't) but want to provide OTLP data.

- Istio is mandatory for mTLS

  App to Gateway communication is across nodes and with that should use mTLS, so Istio needs to be used which sometimes is not possible. Also, Istio causes additional overhead like noise filtering and collecting observability data about the service mesh should not rely on the service mesh.

- Cumulativetodelta transformation not supported

  This transformation is mandatory for Dynatrace support and requires that data from a Pod gets processed always by the same instance.

- General misconception

  Gateways are usually running outside the cluster as a first entry point. The cluster should get rid of the data as fast as possible as persistence cannot be provided here (things like Kafka would again run outside). Try to compensate for short-lived hiccups. Otherwise, point back the backpressure to the client and do not try to solve the big picture.

## Proposal

Most of the drawbacks can be solved by running the gateway logic node-local only. Every instance always processes only data of the local node. With that a natural scalability is given and can be extended by vertical scaling capabilities. The "old" gateways will be running in agent mode but still called "OTLP Gateway". Additionally, there will be a new uniform service `telemetry-otlp` forwarding to the node-local entity using the [internalTrafficPolicy: local](https://kubernetes.io/docs/reference/networking/virtual-ips/#internal-traffic-policy) setting. Existing services can stay compatible using the same approach.

![arch](./../assets/otlp-gateway-new.drawio.svg)

### Remaining Drawbacks of the Agent Approach  

#### Coupling of Signal Types  

Decoupling of signal types will no longer be possible, which may result in scenarios where logs are dropped due to metric overload. However, with proper hardening, this should not pose a significant issue.  

Let's compare a **shared OTLP agent setup** with an **agent-per-signal setup** in terms of robustness. The OTel Collector offers two key mechanisms to prevent overload:

1. **Memory Limiter**  
   - Applied globally, no way to restrict it to a specific pipeline or signal type.
   - Performs periodic checks of memory usage and will begin refusing data and forcing GC to reduce memory consumption when defined limits have been exceeded. It influenced by:
     - Heap allocated object held by the Collector.  It can be the OTLP exporter sending queues or various caches used by different components in the chain (e.g. Kubernetes Attributes Processor informers cache).  
     - Heap allocated objects not being used and can be reclaimed by GC.
   - Memory Limiter can not prevent all kinds of OOM issues. It can only prevent issues caused by ingested telemetry data by refusing it. For example, the issue of the Kubernetes Attributes Processor informer cache consuming excessive memory can not be prevented by Memory Limiter.  Basically, it is a poor man's throttling mechanism that was necessary when using the old Batch Processor. [Read more](./017-fault-tolerant-otel-logging-setup.md).

2. **Refusing Data if OTLP Exporter Queue is Full**  
   - The Batch Processor must not be used (the built-in OTLP Exporter batcher does not have this limitation and can still be used).
   - If the queue is full, new data will be refused.  
   - Each pipeline type has its own queue, allowing different queue lengths to support varying quality-of-service (QoS) levels for different signal types.  

Introducing a shared collector handling OTLP data for all three telemetry types means that only one memory limiter is configured, instead of three separate ones when they are isolated. However, if OTLP exporter queue refusal is correctly set up, the system should never reach the point where the memory limiter starts rejecting data. across all clusters is not solvable by either the old or the new proposed setup.

#### Downtime During Updates  

Unlike deployments, the agent cannot be rolled out in a rolling manner, meaning updates will always result in some downtime. However, this should be mitigated by the retry mechanism in the OpenTelemetry SDK (according to our [tests](../../contributor/pocs/otelcol-downtime/otelcol-downtime.md) it's not yet the case).

### Additional Benefits  

#### Persistent Queue Everywhere  

A small persistent buffer can be introduced in all components using the node file system, enhancing resilience and reliability.  

#### No Istio Dependency  

Neither the application nor its components will depend on Istio anymore. However, the metric agent should still support scraping Istio-instrumented endpoints.  

## Conclusion

The former motivation of the gateway concept turned out to be no longer relevant. Switching to the agent approach solves many problems while introducing very soft drawbacks. The transformation should start immediately :)
