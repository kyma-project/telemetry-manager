# Metrics

> **NOTE:** The feature is not available yet. To understand the current progress, watch this [epic](https://github.com/kyma-project/kyma/issues/13079).


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
1. The backend can run within the cluster.
1. If authentication has been set up, the backend can also run outside the cluster.
1. The metric data is consumed using the backend system.

### Metric Gateway
In a Kyma cluster, the metric gateway is the central component to which all components can send their individual metrics. The gateway collects, enriches, and dispatches the data to the configured backend. The gateway is based on the [OTel Collector](https://opentelemetry.io/docs/collector/) and comes with a concept of pipelines consisting of receivers, processors, and exporters, with which you can flexibly plug pipelines together (see [Configuration](https://opentelemetry.io/docs/collector/configuration/). Kyma's MetricPipeline provides a hardened setup of an OTel Collector and also abstracts the underlying pipeline concept. Such abstraction has the following benefits:
- Supportability - all features are tested and supported
- Migratability - smooth migration experiences when switching underlying technologies or architectures
- Native Kubernetes support - API provided by Kyma supports an easy integration with Secrets, for example, served by the [SAP BTP Service Operator](https://github.com/SAP/sap-btp-service-operator#readme). Telemetry Manager takes care of the full lifecycle.
- Focus - the user doesn't need to understand underlying concepts

The downside is that only a limited set of features is available. If you want to avoid this downside, bring your own collector setup. The current feature set focuses on providing the full configurability of backends integrated by OTLP.

### Metric Agent
If a MetricPipeline configures a feature in the `input.application` section, an additional DaemonSet is deployed acting as an agent. The agent is also based on an [OTel Collector](https://opentelemetry.io/docs/collector/) and encompasses the collection and conversion of Prometheus-based metrics. Hereby, the workload puts an `prometheus.io/scrape` annotation on the specification of the Pod or service, and the agent collects it. The agent pushes all data in OTLP to the central gateway.

### Telemetry Manager
The MetricPipeline resource is managed by Telemetry Manager, a typical Kubernetes [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) responsible for managing the custom parts of the OTel Collector configuration.

![Manager resources](./assets/metrics-resources.drawio.svg)

Telemetry Manager watches all MetricPipeline resources and related Secrets. Whenever the configuration changes, it validates the configuration and generates a new configuration for the gateway and agent, and for each a ConfigMap for the configuration is generated. Referenced Secrets are copied into one Secret that is mounted to the gateway as well.
Furthermore, the manager takes care of the full lifecycle of the Gateway Deployment and the Agent DaemonSet itself. Only if there is a MetricPipeline defined, they are deployed. At anytime, you can opt out of using the feature by not specifying a MetricPipeline.

## Setting up a MetricPipeline

In the following steps, you can see how to construct and deploy a typical MetricPipeline. Learn more about the available [parameters and attributes](resources/05-metricpipeline.md).

### Step 1a. Create a MetricPipeline with an OTLP GRPC output
To ship metrics to a new OTLP output, create a resource of the kind `MetricPipeline`:

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

This configures the underlying OTel Collector of the gateway with a pipeline for metrics. The receiver of the pipeline will be of the OTLP type and be accessible using the `telemetry-otlp-metrics` service. As an exporter, an `otlp` or an `otlphttp` exporter is used, depending on the configured protocol.

### Step 1b. Create a MetricPipeline with an OTLP HTTP output

To use the HTTP protocol instead of the default GRPC, use the `protocol` attribute and ensure that the correct port is configured as part of the endpoint. Typically, port `4317` is used for GRPC and port `4318` for HTTP.
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

### Step 2a: Add authentication details from plain text

To integrate with external systems, you must configure authentication details. At the moment, Basic Authentication and custom headers are supported.

See the following code examples for mTLS, basic, and token-based custom authentication:
<div tabs>
  <details>
    <summary>Mutual TLS</summary>

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
  </details>
  <details>
    <summary>Basic authentication</summary>

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
  </details>
  <details>
    <summary>Token-based with custom headers</summary>

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
            value: "Bearer myToken"
  ```
  </details>
</div>

### Step 2b: Add authentication details from Secrets

Integrations into external systems usually need authentication details dealing with sensitive data. To handle that data properly in Secrets, MetricsPipeline supports the reference of Secrets.

Use the **valueFrom** attribute to map Secret keys as in the following examples:

<div tabs>
  <details>
    <summary>Mutual TLS</summary>

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
  </details>
  <details>
    <summary>Basic authentication</summary>

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
</details>
  <details>
    <summary>Token-based with custom headers</summary>

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
            valueFrom:
              secretKeyRef:
                  name: backend
                  namespace: default
                  key: token 
  ```
  </details>
</div>

The related Secret must have the referenced name, must be located in the referenced Namespace, and must contain the mapped key as in the following example:

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
  token: Bearer YYY
```

### Step 3: Rotate the Secret

Telemetry Manager continuously watches the Secret referenced with the **secretKeyRef** construct. You can update the Secret’s values, and Telemetry Manager detects the changes and applies the new Secret to the setup.
If you use a Secret owned by the [SAP BTP Service Operator](https://github.com/SAP/sap-btp-service-operator), you can configure an automated rotation using a `credentialsRotationPolicy` with a specific `rotationFrequency` and don’t have to intervene manually.

### Step 4: Activate Prometheus-based metrics

> **NOTE:** For the following approach, you must have instrumented your application using a library like the [Prometheus client library](https://prometheus.io/docs/instrumenting/clientlibs/), with a port in your workload exposed serving as a Prometheus metrics endpoint.

To enable collection of Prometheus-based metrics, define a MetricPipeline that has the `prometheus` section enabled as input:
```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  input:
    application:
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

```yaml
prometheus.io/scrape: "true"   # mandatory to enable automatic scraping
prometheus.io/scheme: https    # optional, default is "http" if no Istio sidecar is used. When using a sidecar (Pod has label `security.istio.io/tlsMode=istio`), the default is "https". Use "https" to scrape workloads using Istio client certificates.
prometheus.io/port: "1234"     # optional, configure the port under which the metrics are exposed
prometheus.io/path: /myMetrics # optional, configure the path under which the metrics are exposed
```

> **NOTE:** The agent can scrape endpoints even if the workload is a part of the Istio service mesh and accepts only mTLS communication. However, there's a constraint: scraping through HTTPS is exclusively viable Istio configures the workload using 'STRICT' mTLS mode. If this condition isn't met, scraping via HTTP is the alternative option, achievable by applying the prometheus.io/scheme=http annotation.

### Step 5: Activate runtime metrics
To enable collection of runtime metrics for your Pods, define a MetricPipeline that has the `runtime` section enabled as input:
```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
name: backend
spec:
  input:
    application:
      runtime:
        enabled: true
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
```

The agent will configure the [kubletstatsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/kubeletstatsreceiver) for the metric groups `pod` and `container`. With that, [system metrics](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/kubeletstatsreceiver/documentation.md) related to containers and pods will get collected.

### Step 6: Activate Istio metrics
To enable collection of Istio metrics for your Pods, define a MetricPipeline that has the `istio` section enabled as input:
```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
name: backend
spec:
  input:
    application:
      istio:
        enabled: true
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
```

The agent will start pulling all [Istio metrics](https://istio.io/latest/docs/reference/config/metrics/) from Istio sidecars.

### Step 7: Deploy the Pipeline

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

## Limitations

The metric gateway setup is based on the following assumptions:
- The collector has no autoscaling options and has a limited resource setup of 1 CPU and 1 GiB memory.
- Batching is enabled, and a batch will contain up to 512 metrics/batch.
- A destination can be unavailable for up to 5 minutes without direct loss of metric data.
- An average metric consists of 7 attributes with 64 character length.

This leads to the following limitations:

### Throughput
The maximum throughput is 4200 metric/sec ~= 15.000.000 metrics/hour. If more data must be ingested, it can be refused.

### Unavailability of output
For up to 5 minutes, a retry for data is attempted when the destination is unavailable. After that, data is dropped.

### No guaranteed delivery
The used buffers are volatile. If the gateway or agent instances crash, metric data can be lost.

### Multiple MetricPipeline support
Up to three MetricPipeline resources at a time are supported.

## Troubleshooting

- Symptom: No metrics are arriving at the destination.

   Cause: The backend is not reachable or wrong authentication credentials are used

   Remedy: Investigate the cause with the following steps:
   1. Check the `telemetry-trace-collector` Pods for error logs by calling `kubectl logs -n kyma-system {POD_NAME}`.

- Symptom: Custom metrics don't arrive at the destination, but Istio metrics do.

   Cause: Your SDK version is incompatible with the OTel collector version.
   
   Remedy:
   1. Check which SDK version you are using for instrumentation. 
   1. Investigate whether it is compatible with the OTel collector version.
   1. If required, upgrade to a supported SDK version.
