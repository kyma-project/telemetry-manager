# TracePipeline

The `tracepipeline.telemetry.kyma-project.io` CustomResourceDefinition (CRD) is a detailed description of the kind of data and the format used to filter and ship trace data in Kyma. To get the current CRD and show the output in the YAML format, run this command:

```bash
kubectl get crd tracepipeline.telemetry.kyma-project.io -o yaml
```

## Sample Custom Resource

The following TracePipeline object defines a pipeline that integrates into the local Jaeger instance:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: TracePipeline
metadata:
  name: jaeger
  generation: 1
spec:
  output:
    otlp:
      endpoint:
        value: http://jaeger-collector.jaeger.svc.cluster.local:4317
status:
  conditions:
  - lastTransitionTime: "2024-02-29T01:18:28Z"
    message: Trace gateway Deployment is ready
    observedGeneration: 1
    reason: DeploymentReady
    status: "True"
    type: GatewayHealthy
  - lastTransitionTime: "2024-02-29T01:18:27Z"
    message: ""
    observedGeneration: 1
    reason: ConfigurationGenerated
    status: "True"
    type: ConfigurationGenerated
  - lastTransitionTime: "2024-02-29T01:18:28Z"
    message: '[NOTE: The "Pending" type is deprecated] Trace gateway Deployment is not ready'
    observedGeneration: 1
    reason: TraceGatewayDeploymentNotReady
    status: "False"
    type: Pending
  - lastTransitionTime: "2024-02-29T01:18:28Z"
    message: '[NOTE: The "Running" type is deprecated] Trace gateway Deployment is ready'
    observedGeneration: 1
    reason: TraceGatewayDeploymentReady
    status: "True"
    type: Running
```

For further examples, see the [samples](https://github.com/kyma-project/telemetry-manager/tree/main/config/samples) directory.

## Custom Resource Parameters

For details, see the [TracePipeline specification file](https://github.com/kyma-project/telemetry-manager/blob/main/apis/telemetry/v1alpha1/tracepipeline_types.go).

<!-- The table below was generated automatically -->
<!-- Some special tags (html comments) are at the end of lines due to markdown requirements. -->
<!-- The content between "TABLE-START" and "TABLE-END" will be replaced -->

<!-- TABLE-START -->
### TracePipeline.telemetry.kyma-project.io/v1alpha1

**Spec:**

| Parameter | Type | Description |
| ---- | ----------- | ---- |
| **output** (required) | object | Defines a destination for shipping trace data. Only one can be defined per pipeline. |
| **output.&#x200b;otlp** (required) | object | Configures the underlying Otel Collector with an [OTLP exporter](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/otlpexporter/README.md). If you switch `protocol`to `http`, an [OTLP HTTP exporter](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/otlphttpexporter) is used. |
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

### TracePipeline Status

The status of the TracePipeline is determined by the condition types `GatewayHealthy` and `ConfigurationGenerated`:

> **NOTE:** The condition types `Running` and `Pending` are deprecated and will be removed soon from the status conditions.

| Condition Type         | Condition Status | Condition Reason        | Condition Message                          |
|------------------------|------------------|-------------------------|--------------------------------------------|
| GatewayHealthy         | True             | DeploymentReady         | Trace gateway Deployment is ready          |
| GatewayHealthy         | False            | DeploymentNotReady      | Trace gateway Deployment is not ready      |
| ConfigurationGenerated | True             | ConfigurationGenerated  |                                            |
| ConfigurationGenerated | False            | ReferencedSecretMissing | One or more referenced Secrets are missing |
| ConfigurationGenerated | False            | MaxPipelinesExceeded    | Maximum pipeline count limit exceeded      |
