# Metrics

Observability is all about exposing the internals of the components belonging to an distributed application and making that data analysable at a central place.
While application logs and traces are usually providing request-oriented data, metrics are aggregated statistics exposed by a component to reflect the internal state. Typical statistics like the amount of processed requests, or the amount of registered users, can be very useful to introspect the current state and also the health of a component. Also, you can define proactive and reactive alerts if metrics are about to reach thresholds, or if they already passed thresholds.

The goal of Kyma's Telemetry module is to support you in collecting all relevant metrics of a workload in a Kyma cluster and ship them to a backend for further analysis. Relevant Kyma modules like Istio or Serverless will contribute metrics instantly, and the Telemetry module enriches the data. You can choose among multiple [vendors for OTLP-based backends](https://opentelemetry.io/ecosystem/vendors/).

## Prerequisites

Before you can collect metrics data from a component, it must expose (or instrument) the metrics first. Typically, it instruments specific metrics for the used language runtime (like Node.js) and custom metrics specific to the business logic. Also, the exposure can be in different formats, like the pull-based Prometheus format or the [push-based OTLP format](https://opentelemetry.io/docs/specs/otlp/).

For the instrumentation, you usually use an SDK, namely the [Prometheus client libraries](https://prometheus.io/docs/instrumenting/clientlibs/) or the [Open Telemetry SDKs](https://opentelemetry.io/docs/instrumentation/). Both libraries provide extensions to activate language-specific auto-instrumentation like for Node.js, and an API to implement custom instrumentation.

## Architecture

In the Telemetry module, a central in-cluster Deployment of an [OTel Collector](https://opentelemetry.io/docs/collector/) acts as a gateway. The gateway exposes endpoints for the [OTLP protocol](https://opentelemetry.io/docs/specs/otlp/) for GRPC and HTTP-based communication using the dedicated `telemetry-otlp-metrics` service, to which all Kyma components and users' applications should send the metrics data.

Optionally, the Telemetry module provides a DaemonSet of an [OTel Collector](https://opentelemetry.io/docs/collector/) acting as an agent. That agent pulls metrics of workload in the [Prometheus pull-based format](https://prometheus.io/docs/instrumenting/exposition_formats) and can provide runtime specific metrics for workload.

![Architecture](./assets/metrics-arch.drawio.svg)

1. An application exposing metrics in OTLP, pushes metrics to the central metric gateway service.
2. An application exposing metrics in Prometheus protocol, activates the agent to scrape the metrics with an annotation-based configuration.
3. Additionally, you can activate the agent to pull metrics of each Istio sidecar.
4. The agent converts and pushes all collected metric data to the gateway in OTLP.
5. The gateway enriches all received data with typical metadata of the source by communicating with the Kubernetes APIServer. Furthermore, it filters data according to the pipeline configuration.
6. The `MetricPipeline` resource specifies the target backend for the metric gateway.
7. The backend can run within the cluster.
8. If authentication has been set up, the backend can also run outside the cluster.
9. The metric data is consumed using the backend system.

### Metric Gateway

In a Kyma cluster, the metric gateway is the central component to which all components can send their individual metrics. The gateway collects, enriches, and dispatches the data to the configured backend. For more information, see the [Gateway documentation](./gateways.md).

### Metric Agent

If a MetricPipeline configures a feature in the `input` section, an additional DaemonSet is deployed acting as an agent. The agent is also based on an [OTel Collector](https://opentelemetry.io/docs/collector/) and encompasses the collection and conversion of Prometheus-based metrics. Hereby, the workload puts an `prometheus.io/scrape` annotation on the specification of the Pod or service, and the agent collects it. The agent pushes all data in OTLP to the central gateway.

### Telemetry Manager

The MetricPipeline resource is managed by Telemetry Manager, a typical Kubernetes [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) responsible for managing the custom parts of the OTel Collector configuration.

![Manager resources](./assets/metrics-resources.drawio.svg)

Telemetry Manager watches all MetricPipeline resources and related Secrets. Whenever the configuration changes, it validates the configuration and generates a new configuration for the gateway and agent, and for each a ConfigMap for the configuration is generated. Referenced Secrets are copied into one Secret that is mounted to the gateway as well.
Furthermore, the manager takes care of the full lifecycle of the Gateway Deployment and the Agent DaemonSet itself. Only if there is a MetricPipeline defined, they are deployed. At anytime, you can opt out of using the feature by not specifying a MetricPipeline.

## Setting up a MetricPipeline

In the following steps, you can see how to construct and deploy a typical MetricPipeline. Learn more about the available [parameters and attributes](resources/05-metricpipeline.md).

### Step 1. Create a MetricPipeline

To ship metrics to a new OTLP output, create a resource of the kind `MetricPipeline`. The default protocol is GRPC, but you can choose HTTP instead.

This configures the underlying OTel Collector of the gateway with a pipeline for metrics. The receiver of the pipeline is of the OTLP type and is accessible using the `telemetry-otlp-metrics` service. As an exporter, an `otlp` or an `otlphttp` exporter is used, depending on the configured protocol. Ensure that the correct port is configured as part of the endpoint. Typically, port `4317` is used for GRPC and port `4318` for HTTP.

<!-- tabs:start -->

#### **GRPC**

For GRPC, use:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
```

#### **HTTP**

To use the HTTP protocol, use the `protocol` attribute:
  
```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  output:
    otlp:
      protocol: http
      endpoint:
        value: https://backend.example.com:4318
```

<!-- tabs:end -->

### Step 2a: Add Authentication Details From Plain Text

To integrate with external systems, you must configure authentication details. At the moment, mutual TLS (mTLS), Basic Authentication and custom headers are supported.

<!-- tabs:start -->
  
#### **Mutual TLS**

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  output:
    otlp:
      endpoint:
        value: https://backend.example.com/otlp:4317
      tls:
        cert:
          value: |
            -----BEGIN CERTIFICATE-----
            ...
        key:
          value: |
            -----BEGIN RSA PRIVATE KEY-----
            ...
```

#### **Basic Authentication**

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  output:
    otlp:
      endpoint:
        value: https://backend.example.com/otlp:4317
      authentication:
        basic:
          user:
            value: myUser
          password:
            value: myPwd
```

#### **Add Authentication Details From Plain Text**

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  output:
    otlp:
      endpoint:
        value: https://backend.example.com/otlp:4317
      headers:
        - name: Authorization
          prefix: Bearer
          value: "myToken"
```

<!-- tabs:end -->
### Step 2b: Add Authentication Details From Secrets

Integrations into external systems usually need authentication details dealing with sensitive data. To handle that data properly in Secrets, MetricsPipeline supports the reference of Secrets.

Use the **valueFrom** attribute to map Secret keys as in the following examples:

<!-- tabs:start -->

#### **Mutual TLS**

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  output:
    otlp:
      endpoint:
        value: https://backend.example.com/otlp:4317
      tls:
        cert:
          valueFrom:
            secretKeyRef:
                name: backend
                namespace: default
                key: cert
        key:
          valueFrom:
            secretKeyRef:
                name: backend
                namespace: default
                key: key
```

#### **Basic Authentication**

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  output:
    otlp:
      endpoint:
        valueFrom:
            secretKeyRef:
                name: backend
                namespace: default
                key: endpoint
      authentication:
        basic:
          user:
            valueFrom:
              secretKeyRef:
                name: backend
                namespace: default
                key: user
          password:
            valueFrom:
              secretKeyRef:
                name: backend
                namespace: default
                key: password
```

#### **Token-Based With Custom Headers**

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
      headers:
        - name: Authorization
          prefix: Bearer
          valueFrom:
            secretKeyRef:
                name: backend
                namespace: default
                key: token 
```

<!-- tabs:end -->

The related Secret must have the referenced name, must be located in the referenced namespace, and must contain the mapped key as in the following example:

```yaml
kind: Secret
apiVersion: v1
metadata:
  name: backend
  namespace: default
stringData:
  endpoint: https://backend.example.com:4317
  user: myUser
  password: XXX
  token: YYY
```

The value of the token can be stored in the referenced Secret without any prefix or scheme, and it can be configured in the headers section of the MetricPipeline. In this example, the token has the prefix Bearer.

### Step 3: Rotate the Secret

Telemetry Manager continuously watches the Secret referenced with the **secretKeyRef** construct. You can update the Secret’s values, and Telemetry Manager detects the changes and applies the new Secret to the setup.
If you use a Secret owned by the [SAP BTP Service Operator](https://github.com/SAP/sap-btp-service-operator), you can configure an automated rotation using a `credentialsRotationPolicy` with a specific `rotationFrequency` and don’t have to intervene manually.

### Step 4: Activate Prometheus-Based Metrics

> [!NOTE]
> For the following approach, you must have instrumented your application using a library like the [Prometheus client library](https://prometheus.io/docs/instrumenting/clientlibs/), with a port in your workload exposed serving as a Prometheus metrics endpoint.

To enable collection of Prometheus-based metrics, define a MetricPipeline that has the `prometheus` section enabled as input:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  input:
    prometheus:
      enabled: true
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
```

The agent is configured with a generic scrape configuration, which uses annotations to specify the endpoints to scrape in the cluster.
You only need to have the annotations in place for metrics ingestion to start automatically.

Put the following annotations either to a Service that resolves your metrics port, or directly to the Pod:

| Annotation Key                     | Example Values    | Default Value | Description                                                                                                                                                                                                                                                                                                                                 |
|------------------------------------|-------------------|-------------- |---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `prometheus.io/scrape` (mandatory) | `true`, `false` | None | Controls whether Prometheus will automatically scrape metrics from this target.                                                                                                                                                                                                                                                             |
| `prometheus.io/port` (mandatory)   | `8080`, `9100` | None | Specifies the port where the metrics are exposed.                                                                                                                                                                                                                                                                                           |
| `prometheus.io/path`               | `/metrics`, `/custom_metrics` | `/metrics` | Defines the HTTP path where Prometheus can find metrics data.                                                                                                                                                                                                                                                                               |
| `prometheus.io/scheme`             | `http`, `https` | If Istio is active, `https` is supported; otherwise, only `http` is available. The default scheme is `http` unless an Istio sidecar is present, denoted by the label `security.istio.io/tlsMode=istio`, in which case `https` becomes the default. | Determines the protocol used for scraping metrics — either HTTPS with mTLS or plain HTTP. |

> [!NOTE]
> The agent can scrape endpoints even if the workload is a part of the Istio service mesh and accepts mTLS communication. However, there's a constraint: For scraping through HTTPS, Istio must configure the workload using 'STRICT' mTLS mode. Without 'STRICT' mTLS mode, you can set up scraping through HTTP by applying the `prometheus.io/scheme=http` annotation. For related troubleshooting, see [Log entry: Failed to scrape Prometheus endpoint](#log-entry-failed-to-scrape-prometheus-endpoint).

### Step 5: Activate Runtime Metrics

To enable collection of runtime metrics for your Pods, define a MetricPipeline that has the `runtime` section enabled as input:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  input:
    runtime:
      enabled: true
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
```

The agent configures the [kubletstatsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/kubeletstatsreceiver) for the metric groups `pod` and `container`. With that, [system metrics](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/kubeletstatsreceiver/documentation.md) related to containers and pods get collected.

### Step 6: Activate Istio Metrics

To enable collection of Istio metrics for your Pods, define a MetricPipeline that has the `istio` section enabled as input:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  input:
    istio:
      enabled: true
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
```

The agent will start pulling all [Istio metrics](https://istio.io/latest/docs/reference/config/metrics/) from Istio sidecars.

### Step 7: Deactivate OTLP Metrics

To drop the push-based OTLP metrics that are received by the Metric gateway, define a MetricPipeline that has the `otlp` section disabled as an input:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  input:
    istio:
      enabled: true
    otlp:
      disabled: true
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
```

The agent starts pulling all Istio metrics from Istio sidecars, and the push-based OTLP metrics are dropped. Note that the `otlp` input is enabled by default.

### Step 8: Add Filters

To filter metrics by namespaces, define a MetricPipeline that has the `namespaces` section defined in one of the inputs. For example, you can specify the namespaces from which metrics are collected or the namespaces from which metrics are dropped. Learn more about the available [parameters and attributes](resources/05-metricpipeline.md).

The following example collects runtime metrics only from the `foo` and `bar` namespaces:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  input:
    runtime:
      enabled: true
      namespaces:
        include:
          - foo
          - bar
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
```

The following example collects runtime metrics from all namespaces except `foo` and `bar` namespaces:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  input:
    runtime:
      enabled: true
      namespaces:
        exclude:
          - foo
          - bar
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
```

Note that metrics from system namespaces are excluded by default when a namespace selector for the `prometheus` or `runtime` input is not defined. However, for the `istio` and `otlp` input, metrics from system namespaces are included by default if the namespace selector is not defined.

### Step 9: Enable Diagnostic Metrics

When using the `prometheus` or `istio` input feature of the MetricPipeline, typical scrape metrics are produced for every metric source. These metrics include:
- `up`
- `scrape_duration_seconds`
- `scrape_samples_scraped`
- `scrape_samples_post_metric_relabeling`
- `scrape_series_added`

These are rather technical metrics, useful for debugging and diagnostic purposes. 

To enable diagnostic metrics, define a MetricPipeline that has the `diagnosticMetrics` section defined in inputs `prometheus` or/and `istio`. Learn more about the available [parameters and attributes](resources/05-metricpipeline.md).

The following example collects diagnostic metrics only for input `istio`:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  input:
    istio:
      enabled: true
      diagnosticMetrics:
        enabled: true
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
```

The following example collects diagnostic metrics only for input `prometheus`:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  input:
    prometheus:
      enabled: true
      diagnosticMetrics:
        enabled: true
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
```

Diagnostic metrics are only available for inputs `prometheus` and `istio`. They are disabled by default.

### Step 10: Deploy the Pipeline

To activate the constructed MetricPipeline, follow these steps:

1. Place the snippet in a file named for example `metricpipeline.yaml`.
2. Apply the resource file in your cluster:

    ```bash
    kubectl apply -f metricpipeline.yaml
    ```

### Result

You activated a MetricPipeline and metrics start streaming to your backend. To verify that the pipeline is running, verify that the status of the LogPipeline in your cluster is `Ready`:
    ```bash
    kubectl get metricpipeline
    NAME              STATUS    AGE
    backend           Ready     44s

## Operations

A MetricPipeline creates a Deployment running OTel Collector instances in your cluster. That instances will serve OTLP endpoints and ship received data to the configured backend. The Telemetry module assures that the OTel Collector instances are operational and healthy at any time. The Telemetry module delivers the data to the backend using typical patterns like buffering and retries (see [Limitations](#limitations)). However, there are scenarios where the instances will drop logs because the backend is either not reachable for some duration, or cannot handle the log load and is causing back pressure.

To avoid and detect these scenarios, you must monitor the instances by collecting relevant metrics. For that, a service `telemetry-metric-gateway-metrics` is located in the `kyma-system` namespace. For easier discovery, they have the `prometheus.io` annotation.

The relevant metrics are:

| Name | Threshold | Description |
|---|---|---|
| otelcol_exporter_enqueue_failed_metric_points | total[5m] > 0 | Indicates that new or retried items could not be added to the exporter buffer because the buffer is exhausted. Typically, that happens when the configured backend cannot handle the load on time and is causing back pressure. |
| otelcol_exporter_send_failed_metric_points | total[5m] > 0 | Indicates that items are refused in an non-retryable way like a 400 status |
| otelcol_processor_refused_metric_points | total[5m] > 0 | Indicates that items cannot be received because a processor refuses them. That usually happens when memory of the collector is exhausted because too much data arrived and throttling started.. |

## Limitations

The metric setup is based on the following assumptions:

- A destination can be unavailable for up to 5 minutes without direct loss of metric data (using retries).
- An average metric consists of 20 metric data points and 10 labels.
- Batching is enabled, and a batch contains up to 1024 metrics/batch.

This leads to the following limitations:

### Throughput

The default metric gateway setup has a maximum throughput of 34K metric data points/sec. If more data is sent to the gateway, it is refused. Manual scaling can be used to increase the maximum throughput.

The metric agent setup has a maximum throughput of 14K metric data points/sec per instance. If more data must be ingested, it is refused. If a metric data endpoint emits more than 50.000 metric data points per scrape loop, the metric agent refuses all the data.

### Load Balancing With Istio

To assure availability, the metric gateway runs with multiple instances. If you want to increase the maximum throughput, use manual scaling and enter a higher number of instances.
By design, the connections to the gateway are long-living connections (because OTLP is based on gRPC and HTTP/2). For optimal scaling of the gateway, the clients or applications must balance the connections across the available instances, which is automatically achieved if you use an Istio sidecar. If your application has no Istio sidecar, the data is always sent to one instance of the gateway.

### Unavailability of Output

For up to 5 minutes, a retry for data is attempted when the destination is unavailable. After that, data is dropped.

### No Guaranteed Delivery

The used buffers are volatile. If the gateway or agent instances crash, metric data can be lost.

### Multiple MetricPipeline Support

Up to three MetricPipeline resources at a time are supported.

## Troubleshooting

### No Metrics Arrive at the Destination

Symptom: No metrics arrive at the destination.

Cause: The backend is not reachable or wrong authentication credentials are used.

Remedy:

1. To check the `telemetry-metric-gateway` Pods for error logs, call `kubectl logs -n kyma-system {POD_NAME}`.
2. Fix the errors.

### Only Istio Metrics Arrive at the Destination

Symptom: Custom metrics don't arrive at the destination, but Istio metrics do.

Cause: Your SDK version is incompatible with the OTel collector version.

Remedy:

1. Check which SDK version you are using for instrumentation.
2. Investigate whether it is compatible with the OTel collector version.
3. If required, upgrade to a supported SDK version.

### Log Entry: Failed to Scrape Prometheus Endpoint

Symptom: Custom metrics don't arrive at the destination and the OTel Collector produces log entries "Failed to scrape Prometheus endpoint":

```bash
2023-08-29T09:53:07.123Z	warn	internal/transaction.go:111	Failed to scrape Prometheus endpoint	{"kind": "receiver", "name": "prometheus/app-pods", "data_type": "metrics", "scrape_timestamp": 1693302787120, "target_labels": "{__name__=\"up\", instance=\"10.42.0.18:8080\", job=\"app-pods\"}"}
```

Cause: The workload is not configured to use 'STRICT' mTLS mode. For details, see [Activate Prometheus-based metrics](#step-4-activate-prometheus-based-metrics).

Remedy: You can either set up 'STRICT' mTLS mode or HTTP scraping:

<!-- tabs:start -->

#### **Strict mTLS**

Configure the workload using 'STRICT' mTLS mode (for example, by applying a corresponding PeerAuthentication).

#### **HTTP Scraping**

Set up scraping through HTTP by applying the `prometheus.io/scheme=http` annotation.

<!-- tabs:end -->
