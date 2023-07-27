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
  output:
    otlp:
      endpoint:
        value: https://myBackend:4317
status:
  conditions:
  - lastTransitionTime: "2022-12-13T14:33:27Z"
    reason: OpenTelemetryDeploymentNotReady
    type: Pending
  - lastTransitionTime: "2022-12-13T14:33:28Z"
    reason: OpenTelemetryDeploymentReady
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
| **input.&#x200b;application**  | object | Configures application related scraping. |
| **input.&#x200b;application.&#x200b;istio**  | object | Configures istio scraping. |
| **input.&#x200b;application.&#x200b;istio.&#x200b;enabled**  | boolean | Indicates if Istio scraping is enabled. |
| **input.&#x200b;application.&#x200b;prometheus**  | object | Configures Prometheus scraping. |
| **input.&#x200b;application.&#x200b;prometheus.&#x200b;enabled**  | boolean | If enabled, Pods marked with `prometheus.io/scrape=true` annotation will be scraped. |
| **input.&#x200b;application.&#x200b;runtime**  | object | Configures runtime scraping. |
| **input.&#x200b;application.&#x200b;runtime.&#x200b;enabled**  | boolean | If enabled, workload-related Kubernetes metrics will be scraped. |
| **output**  | object | Configures the metric gateway. |
| **output.&#x200b;otlp** (required) | object | Defines an output using the OpenTelemetry protocol. |
| **output.&#x200b;otlp.&#x200b;authentication**  | object | Defines authentication options for the OTLP output |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic**  | object | Activates `Basic` authentication for the destination providing relevant Secrets. |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;password** (required) | object | Contains the basic auth password or a Secret reference. |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;password.&#x200b;value**  | string | Value that can contain references to Secret values. |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;password.&#x200b;valueFrom**  | object |  |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;password.&#x200b;valueFrom.&#x200b;secretKeyRef**  | object | Refers to a key in a Secret. You must provide `name` and `namespace` of the Secret, as well as the name of the `key`. |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;password.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;key**  | string |  |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;password.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;name**  | string |  |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;password.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;namespace**  | string |  |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;user** (required) | object | Contains the basic auth username or a Secret reference. |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;user.&#x200b;value**  | string | Value that can contain references to Secret values. |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;user.&#x200b;valueFrom**  | object |  |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;user.&#x200b;valueFrom.&#x200b;secretKeyRef**  | object | Refers to a key in a Secret. You must provide `name` and `namespace` of the Secret, as well as the name of the `key`. |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;user.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;key**  | string |  |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;user.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;name**  | string |  |
| **output.&#x200b;otlp.&#x200b;authentication.&#x200b;basic.&#x200b;user.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;namespace**  | string |  |
| **output.&#x200b;otlp.&#x200b;endpoint** (required) | object | Defines the host and port (<host>:<port>) of an OTLP endpoint. |
| **output.&#x200b;otlp.&#x200b;endpoint.&#x200b;value**  | string | Value that can contain references to Secret values. |
| **output.&#x200b;otlp.&#x200b;endpoint.&#x200b;valueFrom**  | object |  |
| **output.&#x200b;otlp.&#x200b;endpoint.&#x200b;valueFrom.&#x200b;secretKeyRef**  | object | Refers to a key in a Secret. You must provide `name` and `namespace` of the Secret, as well as the name of the `key`. |
| **output.&#x200b;otlp.&#x200b;endpoint.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;key**  | string |  |
| **output.&#x200b;otlp.&#x200b;endpoint.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;name**  | string |  |
| **output.&#x200b;otlp.&#x200b;endpoint.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;namespace**  | string |  |
| **output.&#x200b;otlp.&#x200b;headers**  | \[\]object | Defines custom headers to be added to outgoing HTTP or GRPC requests. |
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;name** (required) | string | Defines the header name. |
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;value**  | string | Value that can contain references to Secret values. |
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;valueFrom**  | object |  |
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;valueFrom.&#x200b;secretKeyRef**  | object | Refers to a key in a Secret. You must provide `name` and `namespace` of the Secret, as well as the name of the `key`. |
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;key**  | string |  |
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;name**  | string |  |
| **output.&#x200b;otlp.&#x200b;headers.&#x200b;valueFrom.&#x200b;secretKeyRef.&#x200b;namespace**  | string |  |
| **output.&#x200b;otlp.&#x200b;protocol**  | string | Defines the OTLP protocol (http or grpc). Default is GRPC. |

**Status:**

| Parameter | Type | Description |
| ---- | ----------- | ---- |
| **conditions**  | \[\]object | Defines the trail of MetricPipeline conditions. |
| **conditions.&#x200b;lastTransitionTime**  | string |  |
| **conditions.&#x200b;reason**  | string |  |
| **conditions.&#x200b;type**  | string |  |

<!-- TABLE-END -->
