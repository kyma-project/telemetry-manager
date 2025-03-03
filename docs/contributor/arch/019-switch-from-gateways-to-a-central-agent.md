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

The remaining drawbacks of the agent approach are:

- Coupling of signal types

  The decoupling of the signal types will be lost, potentially leading to situations where the agent drops logs caused by metrics overload. However, with the right hardening that should not be a problem.

- Downtimes on updates

  A rollout of the agent cannot happen in a rolling way as for deployments. So there will be a downtime always which should be compensated by the retry mechanism in the otel-sdk

Additional benefits not outlined yet:

- Persistent queue everywhere

  A small persistent buffer could be used in all components based on the node file system.

- No Istio dependency

  Neither the App nor the components have any Istio dependency anymore. However, the metric agent should still support the scraping of istiofied endpoints.

## Conclusion

The former motivation of the gateway concept turned out to be no longer relevant. Switching to the agent approach solves many problems while introducing very soft drawbacks. The transformation should start immediately :)