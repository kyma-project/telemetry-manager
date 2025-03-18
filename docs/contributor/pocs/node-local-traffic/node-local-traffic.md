# Service with local traffic policy

The PoC tries to understand the behavior of the OTLP push service with [service internal traffic policy](https://kubernetes.io/docs/concepts/services-networking/service-traffic-policy/#using-service-internal-traffic-policy).
The gateways in Kyma Telemetry are exposed using an OTLP service such as `telemetry-otlp-traces`.
The applications running in the cluster will push the telemetry data to this service. This PoC tests the behaviour when this OTLP push service is configured with `internalTrafficPolicy: Local`.
The following criteria were tested:
- If the data is only sent to the DaemonSet running on the same node
- If the DaemonSet is not running, then is the data sent to a DaemonSet running on a different node

## Setup

We deployed the following components:
- [Trace generator and sink](./trace-gen.yaml)
- Trace Agent [Setup](./trace-agent.yaml)


## Tests
Tests were performed in the following way:
### Prerequisites
- Deployed the trace agent with service `internalTrafficPolicy: Local`
- Deployed the trace generator and sink, with the trace generator pushing traces to `telemetry-otlp-traces-local.kyma-system:4317`

### Verifications
- The traces are only pushed to the DaemonSet running on the same node as the trace generator.
- When the DaemonSet is in CrashloopBackoff or in Error state, the traces should not be pushed to the backend.
  - To simulate failure, the configmap was introduced with broken config and the DaemonSet was restarted.


### Results

|       | Trace Generator | Trace Sink | Comments                                                                                                                        |
|-------|-----------------|------------|---------------------------------------------------------------------------------------------------------------------------------|
| Istio | disabled        | enabled    | - Peer authentication is required <br> - Data is sent [ADD PREPOSITION] pod on same node <br> - When pod is not running, then data is not received |
| Istio | enabled         | enabled    | - Data is sent [ADD PREPOSITION] pod on same node <br> - When pod is not running, then data is not received                                        |
| Istio | disabled        | disabled   | - Data is sent [ADD PREPOSITION] pod on same node <br> - When pod is not running, then data is not received                                        |
| Istio | enabled         | disabled   | - Data is sent [ADD PREPOSITION] pod on same node <br> - When pod is not running, then data is not received                                        |


### Summary

When OTLP Service is configured with `internalTrafficPolicy: Local`, then the data is only sent to the DaemonSet running on the same node. If the DaemonSet is not running, then the data is not sent to the DaemonSet running on a different node.


