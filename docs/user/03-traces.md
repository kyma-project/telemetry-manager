# Traces

Observability tools aim to show the big picture, no matter if you're monitoring just a few or many components. In a cloud-native microservice architecture, a user request often flows through dozens of different microservices. Logging and monitoring tools help to track the request's path. However, they treat each component or microservice in isolation. This individual treatment results in operational issues.

[Distributed tracing](https://opentelemetry.io/docs/concepts/observability-primer/#understanding-distributed-tracing) charts out the transactions in cloud-native systems, helping you to understand the application behavior and relations between the frontend actions and backend implementation.

The diagram shows how distributed tracing helps to track the request path:

![Distributed tracing](./assets/traces-intro.drawio.svg)

The goal of the Telemetry Module is to support you in collecting all relevant trace data in a Kyma cluster, enrich them and ship them to a backend for further analysis. Kyma modules like Istio or Serverless contribute traces transparently. You can choose among multiple [vendors for OTLP-based backends](https://opentelemetry.io/ecosystem/vendors/).

## Prerequisites

For a complete recording of a distributed trace, it is [essential](https://www.w3.org/TR/trace-context/#problem-statement) that every involved component is at least propagating the trace context. In Kyma, all components involved in users' requests support the [W3C Trace Context protocol](https://www.w3.org/TR/trace-context), which is a vendor-neutral protocol gaining more and more support by all kinds of vendors and tools. The involved Kyma components are mainly Istio, Serverless, and Eventing.

With that, your application must also propagate the W3C Trace Context for any user-related activity. This can be achieved easily using the [Open Telemetry SDKs](https://opentelemetry.io/docs/instrumentation/) available for all common programming languages. If an application follows that guidance and is part of the Istio Service Mesh, it's already outlined with dedicated span data in the trace data collected by the Kyma telemetry setup.

Furthermore, an application should enrich a trace with additional span data and send these data to the cluster-central telemetry services. That can also be achieved with the help of the mentioned [Open Telemetry SDKs](https://opentelemetry.io/docs/instrumentation/).

## Architecture

The Telemetry module provides an in-cluster central deployment of an [OTel Collector](https://opentelemetry.io/docs/collector/) acting as a gateway. The gateway exposes endpoints for the OTLP protocol for GRPC and HTTP-based communication using the dedicated `telemetry-otlp-traces` service, where all Kyma components and users' applications should send the trace data to.

![Architecture](./assets/traces-arch.drawio.svg)

1. An end-to-end request is triggered and populates across the distributed application. Every involved component propagates the trace context using the [W3C Trace Context](https://www.w3.org/TR/trace-context/) protocol.
2. The involved components that have contributed a new span to the trace send the related span data to the trace gateway using the `telemetry-otlp-traces` service. The communication happens based on the [OTLP](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/otlp.md) protocol either using GRPC or HTTP.
3. The trace gateway enriches the span data with relevant metadata, typical for sources running on Kubernetes, like Pod identifiers.
4. With the `TracePipeline` resource, the trace gateway is configured with a target backend.
5. The backend can run in-cluster.
6. The backend can also run out-cluster, if authentication has been set up.
7. The trace data can be consumed using the backend system.

### Trace Gateway

In a Kyma cluster, the trace gateway is the central component to which all components can send their individual spans. The gateway collects, enriches, and dispatches the data to the configured backend. For more information, see the [Gateway documentation](./gateways.md).

### Telemetry Manager

The TracePipeline resource is managed by Telemetry Manager, a typical Kubernetes [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) responsible for managing the custom parts of the OTel Collector configuration.

![Manager resources](./assets/traces-resources.drawio.svg)

Telemetry Manager watches all TracePipeline resources and related Secrets. Whenever the configuration changes, it validates the configuration and generates a new configuration for OTel Collector, where a ConfigMap for the configuration is generated. Referenced Secrets are copied into one Secret that is mounted to the OTel Collector as well.
Furthermore, the manager takes care of the full lifecycle of the OTel Collector Deployment itself. Only if there is a TracePipeline defined, the collector is deployed. At anytime, you can opt out of using the tracing feature by not specifying a TracePipeline.

## Setting up a TracePipeline

In the following steps, you can see how to construct and deploy a typical TracePipeline. Learn more about the available [parameters and attributes](resources/04-tracepipeline.md).

### Step 1a. Create a TracePipeline With an OTLP GRPC Output

To ship traces to a new OTLP output, create a resource of the kind `TracePipeline`:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: TracePipeline
metadata:
  name: backend
spec:
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
```

This configures the underlying OTel Collector with a pipeline for traces. The receiver of the pipeline will be of the OTLP type and be accessible using the `telemetry-otlp-traces` service. As an exporter, an `otlp` or an `otlphttp` exporter is used, dependent on the configured protocol.

### Step 1b. Create a TracePipeline With an OTLP HTTP Output

To use the HTTP protocol instead of the default GRPC, use the `protocol` attribute and ensure that the correct port is configured as part of the endpoint. Typically, port `4317` is used for GRPC and port `4318` for HTTP.

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: TracePipeline
metadata:
  name: backend
spec:
  output:
    otlp:
      protocol: http
      endpoint:
        value: https://backend.example.com:4318
```

### Step 2. Enable Istio Tracing

> [!WARNING]
> The provided Istio feature uses an API in alpha state, which may change in future releases.

By default, the tracing feature of the Istio module is disabled to avoid increased network utilization if there is no TracePipeline.
To activate the Istio tracing feature with a sampling rate of 5% (for recommendations, see [Istio](#istio)), use a resource similar to the following:

```yaml
apiVersion: telemetry.istio.io/v1alpha1
kind: Telemetry
metadata:
  name: tracing-default
  namespace: istio-system
spec:
  tracing:
  - providers:
    - name: "kyma-traces"
    randomSamplingPercentage: 5.00
```

### Step 3a: Add Authentication Details From Plain Text

To integrate with external systems, you must configure authentication details. At the moment, mutual TLS (mTLS), Basic Authentication and custom headers are supported.

<!-- tabs:start -->

#### **Mutual TLS**

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: TracePipeline
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
kind: TracePipeline
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

#### **Token-Based With Custom Headers**

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: TracePipeline
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

### Step 3b: Add Authentication Details From Secrets

Integrations into external systems usually require authentication details dealing with sensitive data. To handle that data properly in Secrets, TracePipeline supports the reference of Secrets.

Use the **valueFrom** attribute to map Secret keys as in the following examples:

<!-- tabs:start -->

#### **Mutual TLS**

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: TracePipeline
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
kind: TracePipeline
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
kind: TracePipeline
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

The related Secret must have the referenced name and needs to be located in the referenced namespace, and contain the mapped key as in the following example:

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

The value of the token can be stored in the referenced Secret without any prefix or scheme, and it can be configured in the headers section of the TracePipeline. In this example, the token has the prefix Bearer.
### Step 4: Rotate the Secret

Telemetry Manager continuously watches the Secret referenced with the **secretKeyRef** construct. You can update the Secret’s values, and Telemetry Manager detects the changes and applies the new Secret to the setup.
If you use a Secret owned by the [SAP BTP Service Operator](https://github.com/SAP/sap-btp-service-operator), you can configure an automated rotation using a `credentialsRotationPolicy` with a specific `rotationFrequency` and don’t have to intervene manually.

### Step 5: Deploy the Pipeline

To activate the constructed TracePipeline, follow these steps:

1. Place the snippet in a file named for example `tracepipeline.yaml`.
2. Apply the resource file in your cluster:

  ```bash
  kubectl apply -f tracepipeline.yaml
  ```

### Result

You activated a TracePipeline and traces start streaming to your backend. To verify that the pipeline is running, verify that the status of the LogPipeline in your cluster is `Ready`:

  ```bash
  kubectl get tracepipeline
  NAME              STATUS    AGE
  backend           Ready     44s
  ```

## Kyma Modules With Tracing Capabilities

Kyma bundles several modules which are potentially involved in user flows. Applications involved in a distributed trace must propagate the trace context to keep the trace complete. Optionally, they can enrich the trace with custom spans, which requires reporting them to the backend.

### Istio

The Istio module is crucial in distributed tracing because it provides the [ingress gateway](https://istio.io/latest/docs/tasks/traffic-management/ingress/ingress-control/). Usually, this is where external requests enter the cluster scope and are enriched with trace context if it hasn't happened yet. Furthermore, every component that's part of the Istio Service Mesh runs an Istio proxy, which propagates the context properly but also creates span data. If Istio tracing is activated and taking care of trace propagation in your application, you get a complete picture of a trace, because every component automatically contributes span data. Also, Istio tracing is pre-configured to be based on the vendor-neutral [w3c-tracecontext](https://www.w3.org/TR/trace-context/) protocol.

> [!WARNING]
> The provided Istio feature uses an API in alpha state, which may change in future releases.

 The Istio module is configured with an [extension provider](https://istio.io/latest/docs/tasks/observability/telemetry/) called `kyma-traces`. To activate the provider on the global mesh level using the Istio [Telemetry API](https://istio.io/latest/docs/reference/config/telemetry/#Tracing), place a resource to the `istio-system` namespace. The following snippets help setting up the Istio tracing feature:

<!-- tabs:start -->

#### **Extension Provider**

The following example configures all Istio proxies with the `kyma-traces` extension provider, which, by default, reports span data to the trace gateway of the Telemetry module.

```yaml
apiVersion: telemetry.istio.io/v1alpha1
kind: Telemetry
metadata:
  name: tracing-default
  namespace: istio-system
spec:
  tracing:
  - providers:
    - name: "kyma-traces"
```

#### **Sampling Rate**

By default, the sampling rate is configured to 1%. That means that only 1 trace out of 100 traces is reported to the trace gateway, and all others are dropped. The sampling decision itself is propagated as part of the [trace context](https://www.w3.org/TR/trace-context/#sampled-flag) so that either all involved components are reporting the span data of a trace, or none. 

> [!TIP]
> If you increase the sampling rate, you send more data your tracing backend and cause much higher network utilization in the cluster. 
> To reduce costs and performance impacts in a production setup, a very low percentage of around 5% is recommended.

To configure an "always-on" sampling, set the sampling rate to 100%:

```yaml
apiVersion: telemetry.istio.io/v1alpha1
kind: Telemetry
metadata:
  name: tracing-default
  namespace: istio-system
spec:
  tracing:
  - providers:
    - name: "kyma-traces"
    randomSamplingPercentage: 100.00
```

#### **Namespaces or Workloads**

If you need specific settings for individual namespaces or workloads, place additional Telemetry resources. If you don't want to report spans at all for a specific workload, activate the `disableSpanReporting` flag with the selector expression.

```yaml
apiVersion: telemetry.istio.io/v1alpha1
kind: Telemetry
metadata:
  name: tracing-default
  namespace: my-namespace
spec:
  selector:
    matchLabels:
      kubernetes.io/name: "my-app"
  tracing:
  - providers:
    - name: "kyma-traces"
    randomSamplingPercentage: 100.00
```

#### **Trace Context Without Spans**
  
To enable the propagation of the [w3c-tracecontext](https://www.w3.org/TR/trace-context/) only, without reporting any spans (so the actual tracing feature is disabled), you must enable the `kyma-traces` provider with a sampling rate of 0. With this configuration, you get the relevant trace context into the [access logs](https://kyma-project.io/#/istio/user/operation-guides/02-30-enable-istio-access-logs) without any active trace reporting.

  ```yaml
  apiVersion: telemetry.istio.io/v1alpha1
  kind: Telemetry
  metadata:
    name: tracing-default
    namespace: istio-system
  spec:
    tracing:
    - providers:
      - name: "kyma-traces"
      randomSamplingPercentage: 0
  ```

 <!-- tabs:end -->

### Eventing

The Kyma [Eventing](https://kyma-project.io/#/eventing-manager/user/README) module dispatches events from an in- or out-cluster backend to your workload. It leverages the [CloudEvents](https://cloudevents.io/) protocol, which natively supports the [W3C Trace Context](https://www.w3.org/TR/trace-context) propagation. That said, the Eventing component already propagates trace context properly but does not enrich a trace with more advanced span data.

### Serverless

By default, all engines for the [Serverless](https://kyma-project.io/#/serverless-manager/user/README) module integrate the [Open Telemetry SDK](https://opentelemetry.io/docs/reference/specification/metrics/sdk/). With that, trace propagation no longer is your concern, because the used middlewares are configured to automatically propagate the context for chained calls. Because the Telemetry endpoints are configured by default, Serverless also reports custom spans for incoming and outgoing requests. You can [customize Function traces](https://kyma-project.io/#/serverless-manager/user/tutorials/01-100-customize-function-traces) to add more spans as part of your Serverless source code.

## Operations

A TracePipeline creates a Deployment running OTel Collector instances in your cluster. That instances will serve OTLP endpoints and ship received data to the configured backend. The Telemetry module assures that the OTel Collector instances are operational and healthy at any time. The Telemetry module delivers the data to the backend using typical patterns like buffering and retries (see [Limitations](#limitations)). However, there are scenarios where the instances will drop logs because the backend is either not reachable for some duration, or cannot handle the log load and is causing back pressure.

To avoid and detect these scenarios, you must monitor the instances by collecting relevant metrics. For that, a service `telemetry-trace-collector-metrics` is located in the `kyma-system` namespace. For easier discovery, they have the `prometheus.io` annotation.

The relevant metrics are:

| Name | Threshold | Description |
|---|---|---|
| otelcol_exporter_enqueue_failed_spans | total[5m] > 0 | Indicates that new or retried items could not be added to the exporter buffer because the buffer is exhausted. Typically, that happens when the configured backend cannot handle the load on time and is causing back pressure. |
| otelcol_exporter_send_failed_spans | total[5m] > 0 | Indicates that items are refused in an non-retryable way like a 400 status |
| otelcol_processor_refused_spans | total[5m] > 0 | Indicates that items cannot be received anymore because a processor refuses them. Typically, that happens when memory of the collector is exhausted because too much data arrived and throttling started. |

## Limitations

The trace gateway setup is designed using the following assumptions:

- The collector has no autoscaling options yet and has a limited resource setup of 1 CPU and 1 GiB memory.
- Batching is enabled, and a batch will contain up to 512 Spans/batch.
- An unavailability of a destination must be survived for 5 minutes without direct loss of trace data.
- An average span consists of 40 attributes with 64 character length.

This leads to the following limitations:

### Throughput

The maximum throughput is 4200 span/sec ~= 15.000.000 spans/hour. If more data must be ingested, it can result in a refusal of more data.

### Unavailability of Output

For up to 5 minutes, a retry for data is attempted when the destination is unavailable. After that, data is dropped.

### No Guaranteed Delivery

The used buffers are volatile. If the OTel collector instance crashes, trace data can be lost.

### Multiple TracePipeline Support

Up to 3 TracePipelines at a time are supported at the moment.

### System Span Filtering

System-related spans reported by Istio are filtered out without the opt-out option. Here are a few examples of such spans:

- `/healthz` endpoint of a component deployed in the `kyma-system` namespace
- `/metrics` endpoint of a component deployed in the `kyma-system` namespace
- All outgoing spans reported by Grafana

## Troubleshooting

- Symptom: No traces are arriving at the destination.

   Cause: The backend is not reachable or wrong authentication credentials are used.

   Remedy: Investigate the cause with the following steps:
   1. Check the `telemetry-trace-collector` Pods for error logs by calling `kubectl logs -n kyma-system {POD_NAME}`.

- Symptom: Custom spans don't arrive at the destination, but Istio spans do.

   Cause: Your SDK version is incompatible with the OTel collector version.

   Remedy:
   1. Check which SDK version you are using for instrumentation.
   1. Investigate whether it is compatible with the OTel collector version.
   1. If required, upgrade to a supported SDK version.

- Symptom: Trace backend shows fewer traces than you would like to see.

   Cause: By [default](#istio), only 1% of the requests are sent to the trace backend for trace recording.

   Remedy:

   To see more traces in the trace backend, increase the percentage of requests by changing the default settings.
   If you just want to see traces for one particular request, you can manually force sampling.

   To override the default percentage, you deploy a YAML file to an existing Kyma installation.
   To set the value for the **randomSamplingPercentage** attribute, create a values YAML file.
   The following example sets the value to `60`, which means 60% of the requests are sent to tracing backend.
    ```yaml
      apiVersion: telemetry.istio.io/v1alpha1
      kind: Telemetry
      metadata:
        name: kyma-traces
        namespace: istio-system
      spec:
        tracing:
        - providers:
          - name: "kyma-traces"
          randomSamplingPercentage: 60
    ```
