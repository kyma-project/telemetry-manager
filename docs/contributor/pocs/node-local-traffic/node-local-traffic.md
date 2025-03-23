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
- Trace Agent Daemon set [Setup](./trace-agent.yaml)


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

| Trace Generator | Trace Agent    | Data is Sent to Trace Agent running on the same Node | Data is not received at Sink when Trace Agent is not running | comments                        |
|-----------------|----------------|------------------------------------------------------|--------------------------------------------------------------|---------------------------------|
| Istio disabled  | Istio enabled  | yes                                                  | yes                                                          | Peer authentication is required |
| Istio  enabled  | Istio enabled  | yes                                                  | yes                                                          |                                 |
| Istio disabled  | Istio disabled | yes                                                  | yes                                                          |                                 |
| Istio enabled   | Istio disabled | yes                                                  | yes                                                          |                                 |


### Security
While performing the PoC we also tried to understand from the perspective of security. As when the communication is within the same node would it be more secure ? Following is the result of investigation:

To enable Pod-to-Pod communication K8s is creating a virtual network interface in the root network namespace of the node. The virtual interfaces are connected via a virtual network bridge (see https://opensource.com/article/22/6/kubernetes-networking-fundamentals). The connections inside the cluster are always point-to-point connections between the communiation partners. Because of this, the network traffic between Pods cannot be traced easily by a 3rd Pod which is not part of that communication. In order to intercept this traffic, an attacker must have access to the root network namespace of the node or must be able to add a man in the middle between 2 communication partners. This makes it pretty difficult to trace the network communication of 2 Pods on the same node. If the Pods are on 2 different Nodes, an attacker could trace the network traffic between the Pods because the Node-to-Node connection is not encrypted.

Taking the above results into account, the potential risk of a data breach is low.


### Summary

When OTLP Service is configured with `internalTrafficPolicy: Local`, then the data is only sent to the DaemonSet running on the same node. If the DaemonSet is not running, then the data is not sent to the DaemonSet running on a different node.


