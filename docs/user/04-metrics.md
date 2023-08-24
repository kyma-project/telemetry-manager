# Metrics

>**Note**: The feature is not available yet. To understand the current progress, watch this [epic](https://github.com/kyma-project/kyma/issues/13079).


Observability is all about exposing the internals of components in a distributed application and making that internal analyzable at a central place.
While application logs and traces are usually request oriented, metrics are aggregated statistics exposed by a component to reflect the internal state. Typical statistics like the amount of processed requests, or the amount registered users can be very useful to introspect the current state and also the health of a component. Also, it allows to define proactive and reactive alerts in case of metrics are about to reach thresholds soon or in case they already passed thresholds.

The goal of the Telemetry Module is to support you in collecting all relevant metrics of workload in a Kyma cluster and ship them to a backend for further analysis. Hereby, relevant Kyma modules like Istio or Serverless will contribute instantly and typical enrichment of the data is happening. Multiple [vendors for OTLP-based backends](https://opentelemetry.io/ecosystem/vendors/) are available.

## Prerequisites

A component from which you want to collect metrics data, needs to expose (or instrument) the metrics first. Typically, it instruments specific metrics for the used language runtime (like nodeJS) and custom metrics specific to the business logic. Also, the exposure can be in different formats, usually the pull-based prometheus format or the [push-based OTLP format](https://opentelemetry.io/docs/specs/otlp/).

To do the instrumentation you usually leverage an SDK, namely the [prometheus-client libraries](https://prometheus.io/docs/instrumenting/clientlibs/) or the [Open Telemetry SDKs](https://opentelemetry.io/docs/instrumentation/). Both libraries provide extensions to activate language specific auto-instrumentation like for nodeJS and an API to implement custom instrumentation.

## Architecture

The Telemetry module provides an in-cluster central Deployment of an [OTel Collector](https://opentelemetry.io/docs/collector/) acting as a gateway. The gateway exposes endpoints for the [OTLP protocol](https://opentelemetry.io/docs/specs/otlp/) for GRPC and HTTP-based communication using the dedicated `telemetry-otlp-metrics` service, where all Kyma components and users' applications should send the metrics data to.

Optional, it provides a DaemonSet of an [OTel Collector](https://opentelemetry.io/docs/collector/) acting as an agent. That agent will scrape metrics of workload in the [prometheus pull-based format](https://prometheus.io/docs/instrumenting/exposition_formats) and can provide runtime specific metrics for workload.

![Architecture](./assets/metrics-arch.drawio.svg)

1. An application exposing metrics in OTLP, pushes metrics to the central metric gateway service 
2. An application exposing metrics in prometheus protocol, activates the agent to scrape the metrics via aan annotation based configuration
3. The agent can be activated to scrape metrics of each istio sidecar additionally
4. The agent convertes and pushes all scraped metric data to the gateway in OTLP
5. The gateway enrichs all received data with typical metadata of the source by communicating with the K8S APIServer. Furthermore, it will filter data according to the pipeline configuration.
6. With the `MetricPipeline` resource, the metric gateway is configured with a target backend.
1. The backend can run in-cluster.
1. The backend can also run out-cluster, if authentication has been set up.
1. The metric data can be consumed using the backend system.

### Metric Gateway
In a Kyma cluster, the Metric Gateway is the central component to which all components can send their individual metrics. The gateway collects, enriches, and dispatches the data to the configured backend. The gateway is based on the [OTel Collector](https://opentelemetry.io/docs/collector/) and comes with a [concept](https://opentelemetry.io/docs/collector/configuration/) of pipelines consisting of receivers, processors, and exporters, with which you can flexibly plug pipelines together. Kyma's MetricPipeline provides a hardened setup of an OTel Collector and also abstracts the underlying pipeline concept. Such abstraction has the following benefits:
- Supportability - all features are tested and supported
- Migratability - smooth migration experiences when switching underlying technologies or architectures
- Native Kubernetes support - API provided by Kyma allows for an easy integration with Secrets, for example, served by the [SAP BTP Service Operator](https://github.com/SAP/sap-btp-service-operator#readme). Telemetry Manager takes care of the full lifecycle.
- Focus - the user doesn't need to understand underlying concepts

The downside is that only a limited set of features is available. If you want to avoid this downside, bring your own collector setup. The current feature set focuses on providing the full configurability of backends integrated by OTLP.

### Metric Agent
If a MetricPipeline configures a feature in the `input.application` section, an additional DaemonSet gets deployed acting as an agent. The agent is as well based on an [OTel Collector](https://opentelemetry.io/docs/collector/) and is encompassing the collection and conversion of prometheus-based metrics. Hereby, the workload simply puts an typical `prometheus.io/scrape` annotation onn the pod or service specification and the agent will collect scraping it. The agent will push all data in OTLP to the central gateway.
### Telemetry Manager
The MetricPipeline resource is managed by Telemetry Manager, a typical Kubernetes [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) responsible for managing the custom parts of the OTel Collector configuration.

![Manager resources](./assets/metrics-resources.drawio.svg)

Telemetry Manager watches all MetricPipeline resources and related Secrets. Whenever the configuration changes, it validates the configuration and generates a new configuration for the gateway and agent, where for each a ConfigMap for the configuration is generated. Referenced Secrets are copied into one Secret that is mounted to the gateway as well.
Furthermore, the manager takes care of the full lifecycle of the Gateway Deployment and the Agent DaemonSet itself. Only if there is a MetricPipeline defined, they are deployed. At anytime, you can opt out of using the feature by not specifying a MetricPipeline.

## Setting up a MetricPipeline

In the following steps, you can see how to set up a typical MetricPipeline. Learn more about the available [parameters and attributes](resources/05-metricpipeline.md).

### Step 1. Create a MetricPipeline with an output
1. To ship metrics to a new OTLP output, create a resource file of the kind `MetricPipeline`:

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

   This configures the underlying OTel Collector of the gateway with a pipeline for metrics. The receiver of the pipeline will be of the OTLP type and be accessible using the `telemetry-otlp-metrics` service. As an exporter, an `otlp` or an `otlphttp` exporter is used, dependent on the configured protocol.

2. To create the instance, apply the resource file in your cluster:
    ```bash
    kubectl apply -f path/to/my-metric-pipeline.yaml
    ```

3. Check that the status of the MetricPipeline in your cluster is `Ready`:
    ```bash
    kubectl get metricpipeline
    NAME              STATUS    AGE
    http-backend      Ready     44s
    ```

### Step 2. Switch the protocol to HTTP

To use the HTTP protocol instead of the default GRPC, use the `protocol` attribute and ensure that the proper port is configured as part of the endpoint. Typically, port `4317` is used for GRPC and port `4318` for HTTP.
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

### Step 3: Add authentication details

To integrate with external systems, you must configure authentication details. At the moment, Basic Authentication and custom headers are supported.
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

### Step 4: Add authentication details from Secrets

Integrations into external systems usually require authentication details dealing with sensitive data. To handle that data properly in Secrets, MetricsPipeline supports the reference of Secrets.

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

The related Secret must have the referenced name and needs to be located in the referenced Namespace, and contain the mapped key as in the following example:

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

### Step 5: Rotate the Secret

Telemetry Manager continuously watches the Secret referenced with the **secretKeyRef** construct. You can update the Secret’s values, and Telemetry Manager detects the changes and applies the new Secret to the setup.
If you use a Secret owned by the [SAP BTP Service Operator](https://github.com/SAP/sap-btp-service-operator), you can configure an automated rotation using a `credentialsRotationPolicy` with a specific `rotationFrequency` and don’t have to intervene manually.

### Step 6: Activate Prometheus-based metrics

To enable collection of prometheus based metrics define a MetricPipeline having prometheus enabled as input:
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

This approach assumes that you instrumented your application using a library like the [prometheus client library](https://prometheus.io/docs/instrumenting/clientlibs/), having a port in your workload exposed serving a typical Prometheus metrics endpoint.

The agent is configured with a generic scrape configuration, which uses annotations to specify the endpoints to scrape in the cluster. 
Having the annotations in place is everything you need for metrics ingestion to start automatically.

Put the following annotations either to a service that resolves your metrics port, or directly to the pod:

```yaml
prometheus.io/scrape: "true"   # mandatory to enable automatic scraping
prometheus.io/scheme: https    # optional, default is "http" if no Istio sidecar is used. When using a sidecar (Pod has label `security.istio.io/tlsMode=istio`), the default is "https". Use "https" to scrape workloads using Istio client certificates.
prometheus.io/port: "1234"     # optional, configure the port under which the metrics are exposed
prometheus.io/path: /myMetrics # optional, configure the path under which the metrics are exposed
```

> **NOTE:** The agent can scrape endpoints even if the workload uses Istio and accepts only mTLS communication.

### Step 7: Activate runtime metrics
To enable collection of runtime metrics for your pods, define a MetricPipeline having runtime enabled as input:
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

### Step 8: Activate Istio metrics
To enable collection of istio metrics for your pods, define a MetricPipeline having istio enabled as input:
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

## Limitations

The metric gateway setup is designed using the following assumptions:
- The collector has no autoscaling options yet and has a limited resource setup of 1 CPU and 1 GiB memory.
- Batching is enabled, and a batch will contain up to 512 metrics/batch.
- An unavailability of a destination must be survived for 5 minutes without direct loss of metric data.
- An average metric consists of 40 attributes with 64 character length.

This leads to the following limitations:
### Throughput
The maximum throughput is 4200 metric/sec ~= 15.000.000 metrics/hour. If more data must be ingested, it can result in a refusal of more data.

### Unavailability of output
For up to 5 minutes, a retry for data is attempted when the destination is unavailable. After that, data is dropped.

### No guaranteed delivery
The used buffers are volatile. If the gateway or agent instances crash, metric data can be lost.

### Multiple MetricPipeline support

Up to three MetricPipeline resources at a time are supported at the moment.

## Troubleshooting

- Symptom: Traces are not arriving at the destination at all.

   Cause: That might be due to {add reasons}.

   Remedy: Investigate the cause with the following steps:
   1. Check the `telemetry-trace-collector` Pods for error logs by calling `kubectl logs -n kyma-system {POD_NAME}`.
   1. In the monitoring dashboard for Kyma Telemetry, check if the data is exported.
   1. Verify that you activated Istio tracing.

- Symptom: Custom spans don't arrive at the destination, but Istio spans do.

   Cause: Your SDK version is incompatible with the OTel collector version.
   
   Remedy:
   1. Check which SDK version you are using for instrumentation. 
   1. Investigate whether it is compatible with the OTel collector version.
   1. If required, upgrade to a supported SDK version.
