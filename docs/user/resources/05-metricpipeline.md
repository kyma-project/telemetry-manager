# MetricPipeline

The `metricpipeline.telemetry.kyma-project.io` CustomResourceDefinition (CRD) is a detailed description of the kind of data and the format used to filter and ship metric data in Kyma. To get the current CRD and show the output in the YAML format, run this command:

```bash
kubectl get crd metricpipeline.telemetry.kyma-project.io -o yaml
```

## Sample Custom Resource

The following MetricPipeline object defines a pipeline that integrates into an OTLP backend:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: otlp
  generation: 1
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
  - lastTransitionTime: "2024-01-09T07:02:16Z"
    message: "Metric agent DaemonSet is ready"
    observedGeneration: 1
    reason: DaemonSetReady
    status: "True"
    type: AgentHealthy
  - lastTransitionTime: "2024-01-08T10:40:18Z"
    message: "Metric gateway Deployment is ready"
    observedGeneration: 1
    reason: DeploymentReady
    status: "True"
    type: GatewayHealthy
  - lastTransitionTime: "2023-12-28T11:27:04Z"
    message: ""
    observedGeneration: 1
    reason: ConfigurationGenerated
    status: "True"
    type: ConfigurationGenerated
```

For further examples, see the [samples](https://github.com/kyma-project/telemetry-manager/tree/main/config/samples) directory.

## Custom Resource Parameters

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
| **input.&#x200b;istio.&#x200b;diagnosticMetrics**  | object | Configures diagnostic metrics scraping |
| **input.&#x200b;istio.&#x200b;diagnosticMetrics.&#x200b;enabled**  | boolean | If enabled, diagnostic metrics are scraped. The default is `false`. |
| **input.&#x200b;istio.&#x200b;enabled**  | boolean | If enabled, metrics for istio-proxy containers are scraped from Pods that have had the istio-proxy sidecar injected. The default is `false`. |
| **input.&#x200b;istio.&#x200b;namespaces**  | object | Describes whether istio-proxy metrics from specific Namespaces are selected. System Namespaces are enabled by default. |
| **input.&#x200b;istio.&#x200b;namespaces.&#x200b;exclude**  | \[\]string | Exclude metrics from the specified Namespace names only. |
| **input.&#x200b;istio.&#x200b;namespaces.&#x200b;include**  | \[\]string | Include metrics from the specified Namespace names only. |
| **input.&#x200b;otlp**  | object | Configures the collection of push-based metrics that use the OpenTelemetry protocol. |
| **input.&#x200b;otlp.&#x200b;disabled**  | boolean | If disabled, push-based OTLP metrics are not collected. The default is `false`. |
| **input.&#x200b;otlp.&#x200b;namespaces**  | object | Describes whether push-based OTLP metrics from specific Namespaces are selected. System Namespaces are enabled by default. |
| **input.&#x200b;otlp.&#x200b;namespaces.&#x200b;exclude**  | \[\]string | Exclude metrics from the specified Namespace names only. |
| **input.&#x200b;otlp.&#x200b;namespaces.&#x200b;include**  | \[\]string | Include metrics from the specified Namespace names only. |
| **input.&#x200b;prometheus**  | object | Configures Prometheus scraping. |
| **input.&#x200b;prometheus.&#x200b;diagnosticMetrics**  | object | Configures diagnostic metrics scraping |
| **input.&#x200b;prometheus.&#x200b;diagnosticMetrics.&#x200b;enabled**  | boolean | If enabled, diagnostic metrics are scraped. The default is `false`. |
| **input.&#x200b;prometheus.&#x200b;enabled**  | boolean | If enabled, Pods marked with `prometheus.io/scrape=true` annotation are scraped. The default is `false`. |
| **input.&#x200b;prometheus.&#x200b;namespaces**  | object | Describes whether Prometheus metrics from specific Namespaces are selected. System Namespaces are disabled by default. |
| **input.&#x200b;prometheus.&#x200b;namespaces.&#x200b;exclude**  | \[\]string | Exclude metrics from the specified Namespace names only. |
| **input.&#x200b;prometheus.&#x200b;namespaces.&#x200b;include**  | \[\]string | Include metrics from the specified Namespace names only. |
| **input.&#x200b;runtime**  | object | Configures runtime scraping. |
| **input.&#x200b;runtime.&#x200b;enabled**  | boolean | If enabled, workload-related Kubernetes metrics are scraped. The default is `false`. |
| **input.&#x200b;runtime.&#x200b;namespaces**  | object | Describes whether workload-related Kubernetes metrics from specific Namespaces are selected. System Namespaces are disabled by default. |
| **input.&#x200b;runtime.&#x200b;namespaces.&#x200b;exclude**  | \[\]string | Exclude metrics from the specified Namespace names only. |
| **input.&#x200b;runtime.&#x200b;namespaces.&#x200b;include**  | \[\]string | Include metrics from the specified Namespace names only. |
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
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;prefix**  | string | Defines an optional header value prefix. The prefix is separated from the value by a space character. |
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;value**  | string | The value as plain text. |
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;valueFrom**  | object | The value as a reference to a resource. |
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;valueFrom.&#x200b;secretKeyRef**  | object | Refers to the value of a specific key in a Secret. You must provide `name` and `namespace` of the Secret, as well as the name of the `key`. |
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;key**  | string | The name of the attribute of the Secret holding the referenced value. |
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;name**  | string | The name of the Secret containing the referenced value |
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;namespace**  | string | The name of the Namespace containing the Secret with the referenced value. |
| **output.&#x200b;otlp.&#x200b;path**  | string | Defines OTLP export URL path (only for the HTTP protocol). This value overrides auto-appended paths /v1/metrics and /v1/traces |
| **output.&#x200b;otlp.&#x200b;protocol**  | string | Defines the OTLP protocol (http or grpc). Default is grpc. |
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
| **conditions.&#x200b;lastTransitionTime** (required) | string | lastTransitionTime is the last time the condition transitioned from one status to another. This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable. |
| **conditions.&#x200b;message** (required) | string | message is a human readable message indicating details about the transition. This may be an empty string. |
| **conditions.&#x200b;observedGeneration**  | integer | observedGeneration represents the .metadata.generation that the condition was set based upon. For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date with respect to the current state of the instance. |
| **conditions.&#x200b;reason** (required) | string | reason contains a programmatic identifier indicating the reason for the condition's last transition. Producers of specific condition types may define expected values and meanings for this field, and whether the values are considered a guaranteed API. The value should be a CamelCase string. This field may not be empty. |
| **conditions.&#x200b;status** (required) | string | status of the condition, one of True, False, Unknown. |
| **conditions.&#x200b;type** (required) | string | type of condition in CamelCase or in foo.example.com/CamelCase. --- Many .condition.type values are consistent across resources like Available, but because arbitrary conditions can be useful (see .node.status.conditions), the ability to deconflict is important. The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt) |

<!-- TABLE-END -->
### MetricPipeline Status

The status of the MetricPipeline is determined by the condition types `GatewayHealthy`, `AgentHealthy` and `ConfigurationGenerated`:

| Condition Type         | Condition Status | Condition Reason        | Condition Message                          |
|------------------------|------------------|-------------------------|--------------------------------------------|
| GatewayHealthy         | True             | DeploymentReady         | Metric gateway Deployment is ready         |
| GatewayHealthy         | False            | DeploymentNotReady      | Metric gateway Deployment is not ready     |
| AgentHealthy           | True             | AgentNotRequired        |                                            |
| AgentHealthy           | True             | DaemonSetReady          | Metric agent DaemonSet is ready            |
| AgentHealthy           | False            | DaemonSetNotReady       | Metric agent DaemonSet is not ready        |
| ConfigurationGenerated | True             | ConfigurationGenerated  |                                            |
| ConfigurationGenerated | False            | ReferencedSecretMissing | One or more referenced Secrets are missing |
| ConfigurationGenerated | False            | MaxPipelinesExceeded    | Maximum pipeline count limit exceeded      |
