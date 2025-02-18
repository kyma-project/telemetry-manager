# 17. Switch from Gateways to a central Agent

Date: 2025-02-17

## Status

Proposed

## Context

The current architecture defines a set of central Gateways with the single purpose of enriching the data and dispatching it to the backend. That approach had good reasons in the past but a situation is reached were the drawbacks seem to overweight benefits.

![arch](./../assets/otlp-gateway-old.drawio.svg)

The benefits of Gateways are:
- persistent buffering via PV
  The experience showed that persistent buffering on the node filesystem has drawbacks like increased IO and limited space. We always envisioned a configurable persistent size per pipeline which will be only possible in a central StatefulSet having PVs attached. However, experience now shows that operating a StatefulSets with PVs is not trivial and especially on Azure, re-attaching disks can be a very long procedure resulting in long downtimes. Also, persistent buffering might be even a pre-mature optimization with limited gains, see also [ADR-017](./017-fault-tolerant-otel-logging-setup.md).
- separation of concerns
  It is always good to have clean responsibilities to keep code better maintainable. Here, the gateways will do enrichment and dispatching, while agents to data collection only
- decoupling of signal types
  Processing the signal types in separate processes keep the processing tolerant to failures. An overload of metrics will keep the log delivery stable.
- tail-based sampling
  Scenarios like trace sampling requires processing all spans of a trace at one instance, requiring a StatefulSet with sticky trace routing. However, that scenario might not be relevant as this will be done on the backend usually.

The drawback of Gateways are:
- no uniform push endpoint
  by design there is a gateway per signal type were the application need to communicate to. With that you need to configure an individual OTLP endpoint per signal type always. Having an uniform URL will require an additional component route the requests.
- autoscaling is tricky
  The gateway should provide autoscaling capabilities which are tricky to realize. It must be detected very fast if a scale out is needed and it situations were the backend is overloaded must be excluded from scale out.
- every instance caches whole cluster state
  Every instance of a gateway must be able to enrich data of every pod and with that need to cache all pod metadata
- Istio mandatory for load balancing
  Pushing OTLP data from an application to the gateway running with replicas requires loadbalancing. Unfortunately, a kubernetes service is acting on L4 and while GRPC/HTTP2 is on L/, so an additional loadbalancer is needed. That can be easily solved by using Istio, but it becomes a mandatory requirement for applications. However, there can be applications not using Istio (Statefulets often don't do) and like to provide OTLP data.
- Istio is mandatory for mTLS
  App to Gateway communication is across nodes and with that should use mTLS, so Istio needs to be used which sometimes is not possible. Also, Istio causes additional overhead like noise filtering and collecting observability data about the service mesh should not rely on the service mesh.
- cumulativetodelta transformation not suppoerted
  This transformation is mandatory for Dynatrace support and requires that data from a pod gets processed always by the same instance
- General misconception
  Gateways are usually running outside the cluster as a first entry point. The cluster should get rid of the data as fast as possible as persistence cannot be provided here (thinks like Kafka would again run outside). Try to compensate for short-lived hickups but otherwise just point back the backpressure to the client and do not try to solve the big picture

## Proposal

Most of the drawbacks can be solved by running the gateway logic node-local only. Every instance always processes only data of the local node. With that a natural scalability is given and can be extended by vertical scaling capabilities. The "old" gateways will be running in agent mode but still called "OTLP Gateway". Additionally, there will be a new uniform service `telemetry-otlp` forwarding to the node-local entity using the [internalTrafficPolicy: local](https://kubernetes.io/docs/reference/networking/virtual-ips/#internal-traffic-policy) setting. Existing services can stay compatible using the same approach.

![arch](./../assets/otlp-gateway-new.drawio.svg)

The left drawbacks of the agent approach are:
- coupling of signal types
  The decoupling of the signal types will be lost, potentially leading to situations where the agent drops logs caused by metrics overload. However, with the right hardening that should not be a problem
- Downtimes on updates
  A rollout of the agent cannot happen in a rolling way as for deployments. So there will be a downtime always which should be compensated by the retry mechanism in the otel-sdk

Additional benefits not outlined yet
- persistent queue everywhere
  A small persistent buffer could be used in all components based on the node file system
- No Istio dependency
  There is no Istio dependency at all anymore, no at the App and not at the components. However, the metric agent should still support scraping of istiofied endpoints

## Conclusion

The former motivation of the gateway concept turned out to be not that relevant. Switching to an agent approach solves a ton of problems while introducing very soft drawbacks. The transformation should start immediately :)
