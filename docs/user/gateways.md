# Telemetry Gateways

The Telemetry gateways in Kyma take care of data enrichment, filtering, and dispatching, as well as native support for Istio communication.

## Features

Both, the traces and the metrics feature, are based on a gateway, which is provisioned as soon as you define any pipeline resource. All telemetry data of the related domain passes the gateway, so it acts as a central point and provides the following benefits:

- [Data Enrichment](#data-enrichment) to achieve a certain data quality
- Filtering to apply namespace filtering and remove noisy system data (individually for logs, traces, and metrics)
- Dispatching to the configured backends (individually for logs, traces, and metrics)

When the Istio module is added to your Kyma cluster, the gateways support mTLS for the communication from the workload to the gateway, as well as for communication to backends running in the cluster. For details, see [Istio Support](#istio-support).

The gateways are based on the [OTel Collector](https://opentelemetry.io/docs/collector/) and come with a concept of pipelines consisting of receivers, processors, and exporters, with which you can flexibly plug pipelines together (see [Configuration](https://opentelemetry.io/docs/collector/configuration/)). Kyma's MetricPipeline provides a hardened setup of an OTel Collector and also abstracts the underlying pipeline concept. Such abstraction has the following benefits:

- Compatibility: An abstraction layer supports compatibility when underlying features change.
- Migratability: Smooth migration experiences when switching underlying technologies or architectures.
- Native Kubernetes support: API provided by Kyma supports an easy integration with Secrets, for example, served by the [SAP BTP Service Operator](https://github.com/SAP/sap-btp-service-operator#readme). Telemetry Manager takes care of the full lifecycle.
- Focus: The user doesn't need to understand underlying concepts.

The Telemetry module focuses on full configurability of backends integrated by OTLP. If you need more features than provided by the Kyma MetricPipeline, bring your own collector setup.

## Data Enrichment

The Telemetry gateways automatically enrich your data by adding the following attributes:

- `service.name`: The logical name of the service that emits the telemetry data. The gateway ensures that this attribute always has a valid value.
  If not provided by the user, or if its value follows the pattern `unknown_service:<process.executable.name>` as described in the [specification](https://opentelemetry.io/docs/specs/semconv/resource/#service), then it is generated from Kubernetes metadata. The gateway determines the service name based on the following hierarchy of labels and names:
  1. `app.kubernetes.io/name` Pod label value.
  2. `app` Pod label value.
  3. Deployment/DaemonSet/StatefulSet/Job name.
  4. Pod name.
  5. If none of the above is available, the value is `unknown_service`.
- `k8s.*` attributes: These attributes encapsulate various pieces of Kubernetes metadata associated with the Pod, including but not limited to:
  - Pod name
  - Deployment/DaemonSet/StatefulSet/Job name
  - Namespace
  - Cluster name
- Cloud provider attributes: The gateway automatically adds [cloud provider](https://opentelemetry.io/docs/specs/semconv/resource/cloud/) attributes to the telemetry data when data is available. The attribute values are mostly based on the well-known Kubernetes labels available on the nodes.
  - `cloud.provider`: The cloud provider name.
  - `cloud.region`: The region where the node runs. The value is retrieved from node label `topology.kubernetes.io/region`.
  - `cloud.availability_zone`: The zone where the node runs. The value is retrieved from node label `topology.kubernetes.io/zone`.
- Host attributes: The gateway automatically adds [host](https://opentelemetry.io/docs/specs/semconv/resource/host/) attributes to the telemetry data when data is available. The attribute values are based on the well-known Kubernetes labels available on the nodes.
  - `host.type`: The machine type of the node. The value is retrieved from node label `node.kubernetes.io/instance-type`.
  - `host.arch`: The CPU architecture of the system the node is running on. The value is retrieved from node label `kubernetes.io/arch`.

## Istio Support

The Telemetry module automatically detects whether the Istio module is added to your cluster, and injects Istio sidecars to the Telemetry components. Additionally, the ingestion endpoints of gateways are configured to allow traffic in the permissive mode, so they accept mTLS-based communication as well as plain text.

![Gateways-Istio](assets/gateways-istio.drawio.svg)

Clients in the Istio service mesh transparently communicate to the gateway with mTLS. Clients that don't use Istio can communicate with the gateway in plain text mode. The same pattern applies for the communication to the backends running in the cluster. External clusters use the configuration as specified in the pipelines output section.
