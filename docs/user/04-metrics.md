# Metrics

The goal of the Telemetry module is to support you in collecting all relevant metrics of a workload in a Kyma cluster and ship them to a backend for further analysis. Kyma modules like [Istio](https://kyma-project.io/#/istio/user/README) or [Serverless](https://kyma-project.io/#/serverless-manager/user/README) contribute metrics instantly, and the Telemetry module enriches the data. You can choose among multiple [vendors for OTLP-based backends](https://opentelemetry.io/ecosystem/vendors/).

## Overview

Observability is all about exposing the internals of the components belonging to a distributed application and making that data analysable at a central place.
While application logs and traces usually provide request-oriented data, metrics are aggregated statistics exposed by a component to reflect the internal state. Typical statistics like the amount of processed requests, or the amount of registered users, can be very useful to monitor the current state and also the health of a component. Also, you can define proactive and reactive alerts if metrics are about to reach thresholds, or if they already passed thresholds.

## Prerequisites

- Before you can collect metrics data from a component, it must expose (or instrument) the metrics. Typically, it instruments specific metrics for the used language runtime (like Node.js) and custom metrics specific to the business logic. Also, the exposure can be in different formats, like the pull-based Prometheus format or the [push-based OTLP format](https://opentelemetry.io/docs/specs/otlp/).

- If you want to use Prometheus-based metrics, you must have instrumented your application using a library like the [Prometheus client librar](https://prometheus.io/docs/instrumenting/clientlibs/), with a port in your workload exposed serving as a Prometheus metrics endpoint.

- For the instrumentation, you typically use an SDK, namely the [Prometheus client libraries](https://prometheus.io/docs/instrumenting/clientlibs/) or the [Open Telemetry SDKs](https://opentelemetry.io/docs/instrumentation/). Both libraries provide extensions to activate language-specific auto-instrumentation like for Node.js, and an API to implement custom instrumentation.

## Architecture

In the Telemetry module, a central in-cluster Deployment of an [OTel Collector](https://opentelemetry.io/docs/collector/) acts as a gateway. The gateway exposes endpoints for the [OTLP protocol](https://opentelemetry.io/docs/specs/otlp/) for GRPC and HTTP-based communication using the dedicated `telemetry-otlp-metrics` service, to which all Kyma components and users' applications send the metrics data.

Optionally, the Telemetry module provides a DaemonSet of an OTel Collector acting as an agent. This agent can pull metrics of a workload and the Istio sidecar in the [Prometheus pull-based format](https://prometheus.io/docs/instrumenting/exposition_formats) and can provide runtime-specific metrics for the workload.

![Architecture](./assets/metrics-arch.drawio.svg)

1. An application (exposing metrics in OTLP) pushes metrics to the central metric gateway service.
2. Activate the agent to scrape the metrics of an application (exposing metrics in Prometheus protocol) with an annotation-based configuration.
3. Additionally, you can configure the agent to pull metrics of each Istio sidecar.
4. The agent converts and pushes all collected metric data to the gateway in OTLP.
5. The gateway discovers the metadata and enriches all received data with typical metadata of the source by communicating with the Kubernetes APIServer. Furthermore, it filters data according to the pipeline configuration.
6. The `MetricPipeline` resource generates the config for the gateway, which specifies the target backend for the metric gateway.
7. The backend can run within the cluster.
8. If authentication has been set up, the backend can also run outside the cluster.
9. You can analyze the metric data with your preferred backend system.
10. The self monitor observes the metrics flow to the backend and reports problems in the MetricPipeline status.

### Metric Gateway

In a Kyma cluster, the metric gateway is the central component to which all components can send their individual metrics. The gateway collects, enriches, and dispatches the data to the configured backend. For more information, see [Telemetry Gateways](./gateways.md).

### Metric Agent

If a MetricPipeline configures a feature in the `input` section, an additional DaemonSet is deployed acting as an agent. The agent is also based on an [OTel Collector](https://opentelemetry.io/docs/collector/) and encompasses the collection and conversion of Prometheus-based metrics. Hereby, the workload puts a `prometheus.io/scrape` annotation on the specification of the Pod or service, and the agent collects it. The agent pushes all data in OTLP to the central gateway.

### Telemetry Manager

The MetricPipeline resource is watched by Telemetry Manager, which is responsible for generating the custom parts of the OTel Collector configuration.

![Manager resources](./assets/metrics-resources.drawio.svg)

- Telemetry Manager watches all MetricPipeline resources and related Secrets.
- Whenever the configuration changes, it validates the configuration and generates a new configuration for the gateway and agent, and for each, a ConfigMap for the configuration is generated.
- Referenced Secrets are copied into one Secret that is mounted to the gateway as well.
- Furthermore, Telemetry Manager takes care of the full lifecycle of the gateway Deployment and the agent DaemonSet itself. Only if there is a MetricPipeline defined, they are deployed.

If you don't want to use the Metrics feature, simply don't set up a MetricPipeline.

## Setting up a MetricPipeline

In the following steps, you can see how to construct and deploy a typical MetricPipeline. Learn more about the available [parameters and attributes](resources/05-metricpipeline.md).

### 1. Create a MetricPipeline

To ship metrics to a new OTLP output, create a resource of the kind `MetricPipeline` and save the file (named, for example, `metricpipeline.yaml`).

This configures the underlying OTel Collector of the gateway with a pipeline for metrics. It defines that the receiver of the pipeline is of the OTLP type and is accessible with the `telemetry-otlp-metrics` service.

The default protocol is GRPC, but you can choose HTTP instead. Depending on the configured protocol, an `otlp` or an `otlphttp` exporter is used. Ensure that the correct port is configured as part of the endpoint. Typically, port `4317` is used for GRPC and port `4318` for HTTP.

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

For HTTP, use the `protocol` attribute:

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

### 2a. Add Authentication Details From Plain Text

To integrate with external systems, you must configure authentication details. You can use mutual TLS (mTLS), Basic Authentication, or custom headers:

<!-- tabs:start -->

#### **mTLS**

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

#### **Token-based authentication with custom headers**

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
### 2b. Add Authentication Details From Secrets

Integrations into external systems usually need authentication details dealing with sensitive data. To handle that data properly in Secrets, MetricsPipeline supports the reference of Secrets.

Using the **valueFrom** attribute, you can map Secret keys for mutual TLS (mTLS), Basic Authentication, or with custom headers.

You can store the value of the token in the referenced Secret without any prefix or scheme, and you can configure it in the headers section of the MetricPipeline. In this example, the token has the prefix “Bearer”.

<!-- tabs:start -->

#### **mTLS**

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

#### **Token-based authentication with custom headers**

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

The related Secret must have the referenced name, be located in the referenced namespace, and contain the mapped key. See the following example:

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

### 3. Rotate the Secret

Telemetry Manager continuously watches the Secret referenced with the **secretKeyRef** construct. You can update the Secret’s values, and Telemetry Manager detects the changes and applies the new Secret to the setup.

> [!TIP]
> If you use a Secret owned by the [SAP BTP Operator](https://github.com/SAP/sap-btp-service-operator), you can configure an automated rotation using a `credentialsRotationPolicy` with a specific `rotationFrequency` and don’t have to intervene manually.

### 4. Activate Prometheus-Based Metrics

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

The Metric agent is configured with a generic scrape configuration, which uses annotations to specify the endpoints to scrape in the cluster.

For metrics ingestion to start automatically, simply apply the following annotations either to a Service that resolves your metrics port, or directly to the Pod:

| Annotation Key                     | Example Values    | Default Value | Description                                                                                                                                                                                                                                                                                                                                 |
|------------------------------------|-------------------|-------------- |---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `prometheus.io/scrape` (mandatory) | `true`, `false` | None | Controls whether Prometheus automatically scrapes metrics from this target.                                                                                                                                                                                                                                                             |
| `prometheus.io/port` (mandatory)   | `8080`, `9100` | None | Specifies the port where the metrics are exposed.                                                                                                                                                                                                                                                                                           |
| `prometheus.io/path`               | `/metrics`, `/custom_metrics` | `/metrics` | Defines the HTTP path where Prometheus can find metrics data.                                                                                                                                                                                                                                                                               |
| `prometheus.io/scheme`             | `http`, `https` | If Istio is active, `https` is supported; otherwise, only `http` is available. The default scheme is `http` unless an Istio sidecar is present, denoted by the label `security.istio.io/tlsMode=istio`, in which case `https` becomes the default. | Determines the protocol used for scraping metrics — either HTTPS with mTLS or plain HTTP. |

> [!NOTE]
> The agent can scrape endpoints even if the workload is a part of the Istio service mesh and accepts mTLS communication. However, there's a constraint: For scraping through HTTPS, Istio must configure the workload using 'STRICT' mTLS mode. Without 'STRICT' mTLS mode, you can set up scraping through HTTP by applying the annotation `prometheus.io/scheme=http`. For related troubleshooting, see [Log entry: Failed to scrape Prometheus endpoint](#log-entry-failed-to-scrape-prometheus-endpoint).

### 5. Activate Runtime Metrics

To enable collection of runtime metrics, define a MetricPipeline that has the `runtime` section enabled as input:

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

The agent configures the [kubletstatsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/kubeletstatsreceiver) for the metric groups `pod` and `container`. With that, [system metrics](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/kubeletstatsreceiver/documentation.md) related to containers and Pods are collected.

If you want to disable the collection of the Pod or container metrics, define the `resources` section in the `runtime` input.

- The following example drops the runtime Pod metrics and only collects runtime container metrics:

  ```yaml
  apiVersion: telemetry.kyma-project.io/v1alpha1
  kind: MetricPipeline
  metadata:
    name: backend
  spec:
    input:
      runtime:
        enabled: true
        resources:
          pod:
            enabled: false
    output:
      otlp:
        endpoint:
          value: https://backend.example.com:4317
  ```

- The following example drops the runtime container metrics and only collects runtime Pod metrics:

  ```yaml
  apiVersion: telemetry.kyma-project.io/v1alpha1
  kind: MetricPipeline
  metadata:
    name: backend
  spec:
    input:
      runtime:
        enabled: true
        resources:
          container:
            enabled: false
    output:
      otlp:
        endpoint:
          value: https://backend.example.com:4317
  ```

### 6. Activate Istio Metrics

To enable collection of Istio metrics, define a MetricPipeline that has the `istio` section enabled as input:

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

With this, the agent starts pulling all Istio metrics from Istio sidecars.

### 7. Deactivate OTLP Metrics

By default, `otlp` input is enabled.

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

With this, the agent starts pulling all Istio metrics from Istio sidecars, and the push-based OTLP metrics are dropped.

### 8. Add Filters

To filter metrics by namespaces, define a MetricPipeline that has the `namespaces` section defined in one of the inputs. For example, you can specify the namespaces from which metrics are collected or the namespaces from which metrics are dropped. Learn more about the available [parameters and attributes](resources/05-metricpipeline.md).

The following example collects runtime metrics **only** from the `foo` and `bar` namespaces:

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

The following example collects runtime metrics from all namespaces **except** the `foo` and `bar` namespaces:

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

> [!NOTE]
> The default settings depend on the input:
>
> If no namespace selector is defined for the `prometheus` or `runtime` input, then metrics from system namespaces are excluded by default.
>
> However, if the namespace selector is not defined for the `istio` and `otlp` input, then metrics from system namespaces are included by default.

### 9. Activate Diagnostic Metrics

If you use the `prometheus` or `istio` input, for every metric source typical scrape metrics are produced, such as `up`, `scrape_duration_seconds`, `scrape_samples_scraped`, `scrape_samples_post_metric_relabeling`, and `scrape_series_added`.

By default, they are disabled.

If you want to use them for debugging and diagnostic purposes, you can activate them. To activate diagnostic metrics, define a MetricPipeline that has the `diagnosticMetrics` section defined.

- The following example collects diagnostic metrics **only** for input `istio`:

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

- The following example collects diagnostic metrics **only** for input `prometheus`:

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

> [!NOTE]
> Diagnostic metrics are only available for inputs `prometheus` and `istio`. Learn more about the available [parameters and attributes](resources/05-metricpipeline.md).

### 10. Deploy the Pipeline

To activate the MetricPipeline, apply the `metricpipeline.yaml` resource file in your cluster:

```bash
kubectl apply -f metricpipeline.yaml
```

### Result

You activated a MetricPipeline and metrics start streaming to your backend. 

To check that the pipeline is running, wait until all status conditions of the MetricPipeline in your cluster have status `True`:

    ```bash
    kubectl get metricpipeline
    NAME      CONFIGURATION GENERATED   GATEWAY HEALTHY   AGENT HEALTHY   FLOW HEALTHY   AGE
    backend   True                      True              True            True           2m
    ```

## Operations

A MetricPipeline runs several OTel Collector instances in your cluster. This Deployment serves OTLP endpoints and ships received data to the configured backend.

The Telemetry module ensures that the OTel Collector instances are operational and healthy at any time, for example, with buffering and retries. However, there may be situations when the instances drop metrics, or cannot handle the metric load.

To detect and fix such situations, check the pipeline status and check out [Troubleshooting](#troubleshooting).

## Limitations

The metric setup is based on the following assumptions:

- A destination can be unavailable for up to 5 minutes without direct loss of metric data (using retries).
- An average metric consists of 20 metric data points and 10 labels.
- Batching is enabled, and a batch contains up to 1024 metrics/batch.

This leads to the following limitations:

### Throughput

The default metric **gateway** setup has a maximum throughput of 34K metric data points/sec. If more data is sent to the gateway, it is refused. To increase the maximum throughput, manually scale out the gateway by increasing the number of replicas for the Metric gateway.

The metric **agent** setup has a maximum throughput of 14K metric data points/sec per instance. If more data must be ingested, it is refused. If a metric data endpoint emits more than 50.000 metric data points per scrape loop, the metric agent refuses all the data.

### Load Balancing With Istio

To ensure availability, the metric gateway runs with multiple instances. If you want to increase the maximum throughput, use manual scaling and enter a higher number of instances.
By design, the connections to the gateway are long-living connections (because OTLP is based on gRPC and HTTP/2). For optimal scaling of the gateway, the clients or applications must balance the connections across the available instances, which is automatically achieved if you use an Istio sidecar. If your application has no Istio sidecar, the data is always sent to one instance of the gateway.

### Unavailability of Output

For up to 5 minutes, a retry for data is attempted when the destination is unavailable. After that, data is dropped.

### No Guaranteed Delivery

The used buffers are volatile. If the gateway or agent instances crash, metric data can be lost.

### Multiple MetricPipeline Support

Up to three MetricPipeline resources at a time are supported.

## Troubleshooting

### No Metrics Arrive at the Backend

Cause: Incorrect backend endpoint configuration (such as using the wrong authentication credentials) or the backend is unreachable.

Remedy:

- Check the `telemetry-metric-gateway` Pods for error logs by calling `kubectl logs -n kyma-system {POD_NAME}`.
- Check if the backend is up and reachable.

### Not All Metrics Arrive at the Backend

Symptom: The backend is reachable and the connection is properly configured, but some metrics are refused.

Cause: It can happen due to a variety of reasons. For example, a possible reason may be that the backend is limiting the ingestion rate.

Remedy:

1. Check the `telemetry-metric-gateway` Pods for error logs by calling `kubectl logs -n kyma-system {POD_NAME}`. Also, check your observability backend to investigate potential causes.
2. If backend is limiting the rate by refusing metrics, try the options desribed in [Gateway Buffer Filling Up](#gateway-buffer-filling-up).
3. Otherwise, take the actions appropriate to the cause indicated in the logs.

### Only Istio Metrics Arrive at the Backend

Symptom: Custom metrics don't arrive at the backend, but Istio metrics do.

Cause: Your SDK version is incompatible with the OTel collector version.

Remedy:

1. Check which SDK version you are using for instrumentation.
2. Investigate whether it is compatible with the OTel collector version.
3. If required, upgrade to a supported SDK version.

### Log Entry: Failed to Scrape Prometheus Endpoint

Symptom: Custom metrics don't arrive at the destination and the OTel Collector produces log entries "Failed to scrape Prometheus endpoint":

   ```bash
   2023-08-29T09:53:07.123Z warn internal/transaction.go:111 Failed to scrape Prometheus endpoint {"kind": "receiver", "name": "prometheus/app-pods", "data_type": "metrics", "scrape_timestamp": 1693302787120, "target_labels": "{__name__=\"up\", instance=\"10.42.0.18:8080\", job=\"app-pods\"}"}
   ```

Cause: The workload is not configured to use 'STRICT' mTLS mode. For details, see [Activate Prometheus-based metrics](#4-activate-prometheus-based-metrics).

Remedy: You can either set up 'STRICT' mTLS mode or HTTP scraping:

<!-- tabs:start -->

#### **Strict mTLS**

Configure the workload using 'STRICT' mTLS mode (for example, by applying a corresponding PeerAuthentication).

#### **HTTP Scraping**

Set up scraping through HTTP by applying the `prometheus.io/scheme=http` annotation.

<!-- tabs:end -->

### Gateway Buffer Filling Up

Symptom: In the MetricPipeline status, the `TelemetryFlowHealthy` condition has status **BufferFillingUp**.

Cause: The backend export rate is too low compared to the gateway ingestion rate.

Remedy:

- Option 1: Increase maximum backend ingestion rate. For example, by scaling out the SAP Cloud Logging instances.

- Option 2: Reduce emitted metrics by re-configuring the MetricPipeline (for example, by disabling certain inputs or applying namespace filters).

- Option 3: Reduce emitted metrics in your applications.

### Gateway Throttling

Symptom: In the MetricPipeline status, the `TelemetryFlowHealthy` condition has status **GatewayThrottling**.

Cause: Gateway cannot receive metrics at the given rate.

Remedy:

Manually scale out the gateway by increasing the number of replicas for the Metric gateway. See [Module Configuration](https://kyma-project.io/#/telemetry-manager/user/01-manager?id=module-configuration).
