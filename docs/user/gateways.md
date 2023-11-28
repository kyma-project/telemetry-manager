# Gateways

Both the traces and the metrics feature are based on a gateway, which gets provisioned as soon as you define any Pipeline resource. All telemetry data of the related domain will pass the gateway and with that acts as a central point for
- enrichment to achieve a certain data quality (see [Data Enrichement](#data-enrichment))
- filtering to apply namespace filtering and remove noisy system data (realized individually per domain)
- dispatching to the configured backends (realized individually per domain)

When the Istio module is available, the gateway supports mTLS for the communication from the workload to the gateway as well for communication to backends running in the cluster, see [Istio support](#istio-support)

The gateway is based on the [OTel Collector](https://opentelemetry.io/docs/collector/) and comes with a concept of pipelines consisting of receivers, processors, and exporters, with which you can flexibly plug pipelines together (see [Configuration](https://opentelemetry.io/docs/collector/configuration/). Kyma's MetricPipeline provides a hardened setup of an OTel Collector and also abstracts the underlying pipeline concept. Such abstraction has the following benefits:

- Supportability: All features are tested and supported.
- Migratability: Smooth migration experiences when switching underlying technologies or architectures.
- Native Kubernetes support: API provided by Kyma supports an easy integration with Secrets, for example, served by the [SAP BTP Service Operator](https://github.com/SAP/sap-btp-service-operator#readme). Telemetry Manager takes care of the full lifecycle.
- Focus: The user doesn't need to understand underlying concepts.

The downside is that only a limited set of features is available. If you want to avoid this downside, bring your own collector setup. The current feature set focuses on providing the full configurability of backends integrated by OTLP.

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

Clients in the Istio service mesh transparently communicate to the gateway with mTLS. Clients that don't use Istio can communicate with the gateway in plain text mode. The same pattern applies for the communication to the backends running in the cluster. External clusters use the configuration as specified in the pipelines output section.
