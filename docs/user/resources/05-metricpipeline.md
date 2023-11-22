# MetricPipeline

> **Note:** The `metricpipeline.telemetry.kyma-project.io` CustomResourceDefinition is not available yet. To understand the current progress, watch this [epic](https://github.com/kyma-project/kyma/issues/13079).

The `metricpipeline.telemetry.kyma-project.io` CustomResourceDefinition (CRD) is a detailed description of the kind of data and the format used to filter and ship metric data in Kyma. To get the current CRD and show the output in the YAML format, run this command:

```bash
kubectl get crd metricpipeline.telemetry.kyma-project.io -o yaml
```

## Sample custom resource

The following MetricPipeline object defines a pipeline that integrates into an OTLP backend:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: otlp
spec:
  input:
    application:
      prometheus:
        enabled: false
      istio:
        enabled: false
      runtime:
        enabled: false
  output:
    otlp:
      endpoint:
        value: https://myBackend:4317
status:
  conditions:
  - lastTransitionTime: "2022-12-13T14:33:27Z"
    reason: MetricGatewayDeploymentNotReady
    type: Pending
  - lastTransitionTime: "2022-12-13T14:33:28Z"
    reason: MetricGatewayDeploymentReady
    type: Running
```

For further examples, see the [samples](https://github.com/kyma-project/telemetry-manager/tree/main/config/samples) directory.

## Custom resource parameters

For details, see the [MetricPipeline specification file](https://github.com/kyma-project/telemetry-manager/blob/main/apis/telemetry/v1alpha1/metricpipeline_types.go).

<!-- The table below was generated automatically -->
<!-- Some special tags (html comments) are at the end of lines due to markdown requirements. -->
<!-- The content between "TABLE-START" and "TABLE-END" will be replaced -->

<!-- TABLE-START -->
### MetricPipeline.telemetry.kyma-project.io/v1alpha1

**Spec:**

| Parameter | Type | Description |
| ---- | ----------- | ---- |
| **input**  | object | Configures different inputs to send additional metrics to the metric gateway. |
| **input.&#x200b;istio**  | object | Configures istio-proxy metrics scraping. |
| **input.&#x200b;istio.&#x200b;enabled**  | boolean | If enabled, metrics for istio-proxy containers are scraped from Pods that have had the istio-proxy sidecar injected. The default is `false`. |
| **input.&#x200b;istio.&#x200b;namespaces**  | object | Describes whether istio-proxy metrics from specific Namespaces are selected. System namespaces are enabled by default. |
| **input.&#x200b;istio.&#x200b;namespaces.&#x200b;exclude**  | \[\]string | Exclude the Prometheus metrics from the specified Namespace names. |
| **input.&#x200b;istio.&#x200b;namespaces.&#x200b;include**  | \[\]string | Include only the Prometheus metrics from the specified Namespace names. |
| **input.&#x200b;istio.&#x200b;namespaces.&#x200b;system**  | boolean | Set to `true` to include the metrics from system Namespaces like kube-system, istio-system, and kyma-system. |
| **input.&#x200b;otlp**  | object | Configures the collection of push-based metrics which are using the OpenTelemetry protocol. |
| **input.&#x200b;otlp.&#x200b;enabled**  | boolean | If enabled, push-based OTLP metrics are collected. The default is `true`. |
| **input.&#x200b;otlp.&#x200b;namespaces**  | object | Describes whether push-based OTLP metrics from specific Namespaces are selected. System namespaces are disabled by default. |
| **input.&#x200b;otlp.&#x200b;namespaces.&#x200b;exclude**  | \[\]string | Exclude the Prometheus metrics from the specified Namespace names. |
| **input.&#x200b;otlp.&#x200b;namespaces.&#x200b;include**  | \[\]string | Include only the Prometheus metrics from the specified Namespace names. |
| **input.&#x200b;otlp.&#x200b;namespaces.&#x200b;system**  | boolean | Set to `true` to include the metrics from system Namespaces like kube-system, istio-system, and kyma-system. |
| **input.&#x200b;prometheus**  | object | Configures Prometheus scraping. |
| **input.&#x200b;prometheus.&#x200b;enabled**  | boolean | If enabled, Pods marked with `prometheus.io/scrape=true` annotation will be scraped. The default is `false`. |
| **input.&#x200b;prometheus.&#x200b;namespaces**  | object | Describes whether Prometheus metrics from specific Namespaces are selected. System namespaces are disabled by default. |
| **input.&#x200b;prometheus.&#x200b;namespaces.&#x200b;exclude**  | \[\]string | Exclude the Prometheus metrics from the specified Namespace names. |
| **input.&#x200b;prometheus.&#x200b;namespaces.&#x200b;include**  | \[\]string | Include only the Prometheus metrics from the specified Namespace names. |
| **input.&#x200b;prometheus.&#x200b;namespaces.&#x200b;system**  | boolean | Set to `true` to include the metrics from system Namespaces like kube-system, istio-system, and kyma-system. |
| **input.&#x200b;runtime**  | object | Configures runtime scraping. |
| **input.&#x200b;runtime.&#x200b;enabled**  | boolean | If enabled, workload-related Kubernetes metrics will be scraped. The default is `false`. |
| **input.&#x200b;runtime.&#x200b;namespaces**  | object | Describes whether workload-related Kubernetes metrics from specific Namespaces are selected. System namespaces are disabled by default. |
| **input.&#x200b;runtime.&#x200b;namespaces.&#x200b;exclude**  | \[\]string | Exclude the Prometheus metrics from the specified Namespace names. |
| **input.&#x200b;runtime.&#x200b;namespaces.&#x200b;include**  | \[\]string | Include only the Prometheus metrics from the specified Namespace names. |
| **input.&#x200b;runtime.&#x200b;namespaces.&#x200b;system**  | boolean | Set to `true` to include the metrics from system Namespaces like kube-system, istio-system, and kyma-system. |
| **output**  | object | Configures the metric gateway. |
| **output.&#x200b;otlp** (required) | object | Defines an output using the OpenTelemetry protocol. |
| **output.&#x200b;otlp.&#x200b;authentication**  | object | Defines authentication options for the OTLP output |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic**  | object | Activates `Basic` authentication for the destination providing relevant Secrets. |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;password** (required) | object | Contains the basic auth password or a Secret reference. |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;password.&#x200b;value**  | string | The value as plain text. |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;password.&#x200b;valueFrom**  | object | The value as a reference to a resource. |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;password.&#x200b;valueFrom.&#x200b;secretKeyRef**  | object | Refers to the value of a specific key in a Secret. You must provide `name` and `namespace` of the Secret, as well as the name of the `key`. |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;password.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;key**  | string | The name of the attribute of the Secret holding the referenced value. |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;password.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;name**  | string | The name of the Secret containing the referenced value |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;password.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;namespace**  | string | The name of the Namespace containing the Secret with the referenced value. |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;user** (required) | object | Contains the basic auth username or a Secret reference. |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;user.&#x200b;value**  | string | The value as plain text. |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;user.&#x200b;valueFrom**  | object | The value as a reference to a resource. |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;user.&#x200b;valueFrom.&#x200b;secretKeyRef**  | object | Refers to the value of a specific key in a Secret. You must provide `name` and `namespace` of the Secret, as well as the name of the `key`. |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;user.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;key**  | string | The name of the attribute of the Secret holding the referenced value. |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;user.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;name**  | string | The name of the Secret containing the referenced value |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;user.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;namespace**  | string | The name of the Namespace containing the Secret with the referenced value. |
| **output.&#x200b;otlp.&#x200b;endpoint** (required) | object | Defines the host and port (<host>:<port>) of an OTLP endpoint. |
| **output.&#x200b;otlp.&#x200b;endpoint.&#x200b;value**  | string | The value as plain text. |
| **output.&#x200b;otlp.&#x200b;endpoint.&#x200b;valueFrom**  | object | The value as a reference to a resource. |
| **output.&#x200b;otlp.&#x200b;endpoint.&#x200b;valueFrom.&#x200b;secretKeyRef**  | object | Refers to the value of a specific key in a Secret. You must provide `name` and `namespace` of the Secret, as well as the name of the `key`. |
| **output.&#x200b;otlp.&#x200b;endpoint.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;key**  | string | The name of the attribute of the Secret holding the referenced value. |
| **output.&#x200b;otlp.&#x200b;endpoint.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;name**  | string | The name of the Secret containing the referenced value |
| **output.&#x200b;otlp.&#x200b;endpoint.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;namespace**  | string | The name of the Namespace containing the Secret with the referenced value. |
| **output.&#x200b;otlp.&#x200b;headers**  | \[\]object | Defines custom headers to be added to outgoing HTTP or GRPC requests. |
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;name** (required) | string | Defines the header name. |
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;value**  | string | The value as plain text. |
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;valueFrom**  | object | The value as a reference to a resource. |
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;valueFrom.&#x200b;secretKeyRef**  | object | Refers to the value of a specific key in a Secret. You must provide `name` and `namespace` of the Secret, as well as the name of the `key`. |
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;key**  | string | The name of the attribute of the Secret holding the referenced value. |
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;name**  | string | The name of the Secret containing the referenced value |
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;namespace**  | string | The name of the Namespace containing the Secret with the referenced value. |
| **output.&#x200b;otlp.&#x200b;protocol**  | string | Defines the OTLP protocol (http or grpc). Default is GRPC. |
| **output.&#x200b;otlp.&#x200b;tls**  | object | Defines TLS options for the OTLP output. |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;ca**  | object | Defines an optional CA certificate for server certificate verification when using TLS. The certificate must be provided in PEM format. |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;ca.&#x200b;value**  | string | The value as plain text. |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;ca.&#x200b;valueFrom**  | object | The value as a reference to a resource. |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;ca.&#x200b;valueFrom.&#x200b;secretKeyRef**  | object | Refers to the value of a specific key in a Secret. You must provide `name` and `namespace` of the Secret, as well as the name of the `key`. |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;ca.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;key**  | string | The name of the attribute of the Secret holding the referenced value. |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;ca.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;name**  | string | The name of the Secret containing the referenced value |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;ca.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;namespace**  | string | The name of the Namespace containing the Secret with the referenced value. |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;cert**  | object | Defines a client certificate to use when using TLS. The certificate must be provided in PEM format. |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;cert.&#x200b;value**  | string | The value as plain text. |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;cert.&#x200b;valueFrom**  | object | The value as a reference to a resource. |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;cert.&#x200b;valueFrom.&#x200b;secretKeyRef**  | object | Refers to the value of a specific key in a Secret. You must provide `name` and `namespace` of the Secret, as well as the name of the `key`. |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;cert.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;key**  | string | The name of the attribute of the Secret holding the referenced value. |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;cert.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;name**  | string | The name of the Secret containing the referenced value |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;cert.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;namespace**  | string | The name of the Namespace containing the Secret with the referenced value. |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;insecure**  | boolean | Defines whether to send requests using plaintext instead of TLS. |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;insecureSkipVerify**  | boolean | Defines whether to skip server certificate verification when using TLS. |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;key**  | object | Defines the client key to use when using TLS. The key must be provided in PEM format. |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;key.&#x200b;value**  | string | The value as plain text. |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;key.&#x200b;valueFrom**  | object | The value as a reference to a resource. |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;key.&#x200b;valueFrom.&#x200b;secretKeyRef**  | object | Refers to the value of a specific key in a Secret. You must provide `name` and `namespace` of the Secret, as well as the name of the `key`. |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;key.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;key**  | string | The name of the attribute of the Secret holding the referenced value. |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;key.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;name**  | string | The name of the Secret containing the referenced value |
| **output.&#x200b;otlp.&#x200b;tls.&#x200b;key.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;namespace**  | string | The name of the Namespace containing the Secret with the referenced value. |

**Status:**

| Parameter | Type | Description |
| ---- | ----------- | ---- |
| **conditions**  | \[\]object | An array of conditions describing the status of the pipeline. |
| **conditions.&#x200b;lastTransitionTime**  | string | Point in time the condition transitioned into a different state. |
| **conditions.&#x200b;reason**  | string | Reason of last transition. |
| **conditions.&#x200b;type**  | string | The possible transition types are:<br>- `Running`: The instance is ready and usable.<br>- `Pending`: The pipeline is being activated. |

<!-- TABLE-END -->
