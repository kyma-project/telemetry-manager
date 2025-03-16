# Service with local traffic policy

The PoC tries to understand the behavior of the OTLP push service with local traffic policy. The gateways
in Kyma Telemetry are exposed via OTLP service eg: `telemetry-otlp-traces`. This is the service which
applications running in the cluster will push the telemetry data to. In the PoC we test the behaviour when this otlp 
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
- Deployed the trace generator and sink. Where sink would push logs to `telemetry-otlp-traces-local.kyma-system:4317`

### Verifications
- the traces would be pushed only to the daemonSet running on the same node as the trace generator
- When the daemonSet is not running on the same node, the traces should not be pushed to the backend.
  - To simulate failure the configmap was introduced with broken config and the daemonSet was restarted.


### Results

|       | Trace Generator | Trace Sink | Comments                                                                                                                        |
|-------|-----------------|------------|---------------------------------------------------------------------------------------------------------------------------------|
| Istio | disabled        | enabled    | - Peer authentication is required <br> - Data is sent pod on same node <br> - When pod is not running then data is not received |
| Istio | enabled         | enabled    | - Data is sent pod on same node <br> - When pod is not running then data is not received                                        |
| Istio | disabled        | disabled   | - Data is sent pod on same node <br> - When pod is not running then data is not received                                        |
| Istio | enabled         | disabled   | - Data is sent pod on same node <br> - When pod is not running then data is not received                                        |


### Summary


