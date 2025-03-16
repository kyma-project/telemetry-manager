# Service with local traffic policy

The PoC tries to understand the behavior of the OTLP push service with [service internal traffic policy](https://kubernetes.io/docs/concepts/services-networking/service-traffic-policy/#using-service-internal-traffic-policy). The gateways
in Kyma Telemetry are exposed via OTLP service eg: `telemetry-otlp-traces`. the
applications running in the cluster will push the telemetry data to this service. This PoC tests the behaviour when this OTLP 
push service is configured with `internalTrafficPolicy: Local`. Following criteria were tested:
- If the data is only sent to the daemonSet running on the same node
- If the daemonSet for some reason is not running then is the data sent to a daemonSet running on a different node

## Setup

For the setup we deployed following:
- A [trace generator and sink](./trace-gen.yaml)
- Trace Agent [Setup](./trace-agent.yaml)


## Tests
Tests were performed in following way:
### Pre-requisites
- Deployed the trace agent with service `internalTrafficPolicy: Local`
- Deployed the trace generator and sink. Where trace generator would push traces to `telemetry-otlp-traces-local.kyma-system:4317`

### Verifications
- The traces are only pushed to the daemonSet running on the same node as the trace generator
- When the daemonSet is in CrashloopBackoff or in Error state, the traces should not be pushed to the backend.
  - To simulate failure the configmap was introduced with broken config and the daemonSet was restarted.


### Results

|       | Trace Generator | Trace Sink | Comments                                                                                                                        |
|-------|-----------------|------------|---------------------------------------------------------------------------------------------------------------------------------|
| Istio | disabled        | enabled    | - Peer authentication is required <br> - Data is sent pod on same node <br> - When pod is not running then data is not received |
| Istio | enabled         | enabled    | - Data is sent pod on same node <br> - When pod is not running then data is not received                                        |
| Istio | disabled        | disabled   | - Data is sent pod on same node <br> - When pod is not running then data is not received                                        |
| Istio | enabled         | disabled   | - Data is sent pod on same node <br> - When pod is not running then data is not received                                        |


### Summary

When Service is configured with `internalTrafficPolicy: Local` then the data is only sent to the daemonSet running on the same node. If the daemonSet is not running then the data is not sent to the daemonSet running on a different node. If the
daemonSet is in CrashloopBackoff or in Error state then the data is not sent to a different daemonSet running on the different node.

