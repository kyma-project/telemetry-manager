# Gateways

Both the traces and the metrics feature are based on a gateway, which gets provisioned as soon as you define any Pipeline resource. All telemetry data of the related domain will pass the gateway and with that acts as a central point for
- enrichment to achieve a certain data quality (see [Data Enrichement](#data-enrichment))
- filtering to apply namespace filtering and remove noisy system data (realized individually per domain)
- dispatching to the configured backends (realized individually per domain)

When the Istio module is available, the gateway supports mTLS for the communication from the workload to the gateway as well for communication to backends running in the cluster, see [Istio support](#istio-support)

## Data Enrichment

Kyma's Telemetry module automatically enriches your data by adding the following attributes:

- `service.name`: The logical name of the service that emits the telemetry data. If not provided by the user, it is populated from Kubernetes metadata, based on the following hierarchy of labels and names:
  1. `app.kubernetes.io/name` Pod label value.
  2. `app` Pod label value.
  3. Deployment/DaemonSet/StatefulSet/Job name.
  4. Pod name.
  5. If none of the above is available, the value is `unknown_service`.
- `k8s.*` attributes: These attributes encapsulate various pieces of Kubernetes metadata associated with the Pod, including but not limited to:
  1. Pod name.
  2. Deployment/DaemonSet/StatefulSet/Job name.
  3. Namespace.
  4. Cluster name.

## Istio support

The Telemetry module will detect automatically is Istio is available and will inject Istio sidecars to it's components. Additionally, the ingestion endpoints of gateways are configured to allow traffic in the permissive mode, so it will accept mTLS based communication as well as plain text.

![Gateways-Istio](assets/gateways-istio.drawio.svg)

Clients being in the Istio service mesh will transparently communicate to the Gateway using mTLS. Clients not leveraging Istio will still be able to communicate. The same pattern applies for the communication to the backends.
