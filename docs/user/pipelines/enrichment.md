# Data Enrichment

The Telemetry pipelines automatically enrich your data with OTel resource attributes, which makes it easy for you to identify the source of the data in your backend.

## Service Name

Service name is the logical name of the service that emits the telemetry data. The gateway ensures that this attribute always has a valid value.

If not provided by the user, or if its value follows the pattern unknown_service:<process.executable.name> as described in the specification, then it is generated from Kubernetes metadata.

The gateway determines the `service.name` attribute based on the following hierarchy of labels and names:

1. `app.kubernetes.io/name`: Pod label value
1. `app`: Pod label value
1. Deployment/DaemonSet/StatefulSet/Job name
1. Pod name
1. If none of the above is available, the value is `unknown_service`

## Kubernetes Metadata

`k8s.*` attributes encapsulate various pieces of Kubernetes metadata associated with the Pod, such as:

- `k8s.pod.name`: The Kubernetes Pod name of the Pod that emitted the data
- `k8s.pod.uid`: The Kubernetes Pod id of the Pod that emitted the data
- `k8s.<workload kind>.name`: The Kubernetes workload name to which the emitting Pod belongs. Workload is either Deployment, DaemonSet, StatefulSet, Job or CronJob
- `k8s.namespace.name`: The Kubernetes namespace name with which the emitting Pod is associated
- `k8s.cluster.name`: A logical identifier of the cluster, which, by default, is the API Server URL. Users can set a custom name by configuring the `enrichments.cluster.name` field in the [Telemetry CRD](./../resources/01-telemetry.md)
- `k8s.cluster.uid`: A unique identifier of the cluster, realized by the UID of the "kube-system" namespace
- `k8s.node.name`: The Kubernetes node name to which the emitting Pod is scheduled.
- `k8s.node.uid`: The Kubernetes Node id to which the emitting Pod belongs

## Pod Label Attributes

Telemetry pipelines support user-defined enrichments of telemetry data based on Pod labels (see [Telemetry CRD](./../resources/01-telemetry.md)). By configuring specific label keys or label key prefixes to include in the enrichment process, you can capture custom application metadata that may be relevant for filtering, grouping, or correlation purposes. All matching Pod labels are added to the telemetry data as resource attributes, using the label key format `k8s.pod.label.<label_key>`.

The following example configuration enriches the telemetry data with Pod labels that match the specified keys or key prefixes:

- `k8s.pod.label.app.kubernetes.io/name`: The value of the exact label key `app.kubernetes.io/name` from the Pod.
- `k8s.pod.label.app.kubernetes.io.*`: All labels that start with the prefix `app.kubernetes.io` from the Pod, where `*` is replaced by the actual label key.

```yaml
apiVersion: operator.kyma-project.io/v1alpha1
kind: Telemetry
metadata:
  name: default
  namespace: kyma-system
spec:
  enrichments:
    extractPodLabels:
    - key: "<myExactLabelKey>" # for example, "app.kubernetes.io/name"
    - keyPrefix: "<myLabelPrefix>" # for example, "app.kubernetes.io"
  ```

## Cloud Provider Attributes

If data is available, the gateway automatically adds [cloud provider](https://opentelemetry.io/docs/specs/semconv/resource/cloud/) attributes to the telemetry data.

- `cloud.provider`: Cloud provider name
- `cloud.region`: Region where the Node runs (from Node label `topology.kubernetes.io/region`)
- `cloud.availability_zone`: Zone where the Node runs (from Node label `topology.kubernetes.io/zone`)

## Host Attributes

If data is available, the gateway automatically adds [host](https://opentelemetry.io/docs/specs/semconv/resource/host/) attributes to the telemetry data:

- `host.type`: Machine type of the Node (from Node label `node.kubernetes.io/instance-type`)
- `host.arch`: CPU architecture of the system the Node is running on (from Node label `kubernetes.io/arch`)
