# Support Metric Agent for Istio Outbound Traffic

Currently, the Metric Agent bypasses Istio outbound traffic by default. Outbound traffic to in-cluster mesh services is managed through the annotation `traffic.sidecar.istio.io/includeOutboundPorts`, which specifies a list of allowed ports.
Since in-cluster mesh services can use any viable port, the Metric Agent should support in-cluster service discovery and allow the dynamic addition of ports to the list of permitted outbound ports.

## Implementation

In-cluster service discovery can be implemented using two steps:

1. **Endpoint URL check**: Check the configured endpoint URL against Kubernetes DNS to determine whether the service is in-cluster. This approach works when the URL is provided in the format `http://<service-name>.<namespace>.svc.cluster.local:<port>`.
2. **DNS lookup**: When the URL is not in fully qualified domain name (FQDN) format (e.g., `http://<service-name>:<port>`), the `nslookup` utility can be used to resolve the service name to its IP address and verify whether it belongs to the cluster CIDR (RFC1918 or RFC6598) range.

If a URL meets the requirements listed above, the Metric Agent should add the corresponding port to the list of allowed outbound ports. This can be accomplished by updating the `traffic.sidecar.istio.io/includeOutboundPorts` annotation on the pod.

The [PoC](cluster_service_discovery.go) demonstrates how to implement this functionality by checking the endpoint URL.

