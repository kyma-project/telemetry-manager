# Traces

The Telemetry module supports you in collecting all relevant trace data in a Kyma cluster, enriches them and ships them to a backend for further analysis.

## Overview

The Telemetry module provides a trace gateway for the shipment of traces of any container running in the Kyma runtime. Kyma modules like Istio or Serverless contribute traces transparently and the Telemetry module enriches the data. You can choose among multiple [vendors for OTLP-based backends](https://opentelemetry.io/ecosystem/vendors/).

You can configure the trace gateway with external systems using runtime configuration with a dedicated Kubernetes API ([CRD](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions)) named TracePipeline. Additional to the regular [pipeline features](./../pipelines/README.md), the following features are offered by TracePipelines:

- Activate Istio tracing. For details, see [Istio](#istio).
- Collect traces from [Eventing](#eventing) and [Serverless Functions](#serverless).

For an example, see [Sample TracePipeline](./sample.md) and check out the available parameters and attributes under [TracePipeline](./../resources/04-tracepipeline.md).

The Trace feature is optional. If you don't want to use it, simply don't set up a TracePipeline. For details, see Creating a Telemetry Pipeline.

## Prerequisites

For the recording of a distributed trace, every involved component must propagate at least the trace context. For details, see [Trace Context](https://www.w3.org/TR/trace-context/#problem-statement).

- In Kyma, all modules involved in users’ requests support the [W3C Trace Context](https://www.w3.org/TR/trace-context) protocol. The involved Kyma modules are, for example, Istio, Serverless, and Eventing.
- Your application also must propagate the W3C Trace Context for any user-related activity. This can be achieved easily using the [Open Telemetry SDKs](https://opentelemetry.io/docs/instrumentation/) available for all common programming languages. If your application follows that guidance and is part of the Istio Service Mesh, it’s already outlined with dedicated span data in the trace data collected by the Kyma telemetry setup.
- Furthermore, your application must enrich a trace with additional span data and send these data to the cluster-central telemetry services. You can achieve this with [Open Telemetry SDKs](https://opentelemetry.io/docs/instrumentation/).

## Architecture

More details can be found at [Architecture](architecture.md).

## Basic Pipeline

The minimal pipeline will define an [`otlp` output](./../pipelines/otlp-output.md) and have the [`otlp` input](./../pipelines/otlp-input.md) enabled by default.

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: TracePipeline
metadata:
  name: backend
output:
    otlp:
      endpoint:
        value: http://myEndpoint:4317
```

This example will accept traffic from the OTLP metric endpoint filtered by system namespaces and will forward them to the configured backend. The following push URLs are set up:

- GRPC: `http://telemetry-otlp-traces.kyma-system:4317`
- HTTP: `http://telemetry-otlp-traces.kyma-system:4318`

The default protocol for shipping the data to a backend is GRPC, but you can choose HTTP instead. Ensure that the correct port is configured as part of the endpoint.

## Kyma Modules With Tracing Capabilities

Kyma bundles several modules that can be involved in user flows. Applications involved in a distributed trace must propagate the trace context to keep the trace complete. Optionally, they can enrich the trace with custom spans, which requires reporting them to the backend.

### Istio

The Istio module is crucial in distributed tracing because it provides the [Ingress Gateway](https://istio.io/latest/docs/tasks/traffic-management/ingress/ingress-control/). Typically, this is where external requests enter the cluster scope and are enriched with trace context if it hasn’t happened earlier. Furthermore, every component that’s part of the Istio Service Mesh runs an Istio proxy, which propagates the context properly but also creates span data. If Istio tracing is activated and taking care of trace propagation in your application, you get a complete picture of a trace, because every component automatically contributes span data. Also, Istio tracing is pre-configured to be based on the vendor-neutral [W3C Trace Context](https://www.w3.org/TR/trace-context/) protocol.

The Istio module is configured with an [extension provider](https://istio.io/latest/docs/tasks/observability/telemetry/) called `kyma-traces`. To activate the provider on the global mesh level using the Istio [Telemetry API](https://istio.io/latest/docs/reference/config/telemetry/#Tracing), place a resource to the `istio-system` namespace. The following code samples help setting up the Istio tracing feature:

<!-- tabs:start -->

#### **Extension Provider**

The following example configures all Istio proxies with the `kyma-traces` extension provider, which, by default, reports span data to the trace gateway of the Telemetry module.

```yaml
apiVersion: telemetry.istio.io/v1
kind: Telemetry
metadata:
  name: mesh-default
  namespace: istio-system
spec:
  tracing:
  - providers:
    - name: "kyma-traces"
    randomSamplingPercentage: 5.00
```

#### **Sampling Rate**

By default, the sampling rate is configured to 1%. That means that only 1 trace out of 100 traces is reported to the trace gateway, and all others are dropped. The sampling decision itself is propagated as part of the [trace context](https://www.w3.org/TR/trace-context/#sampled-flag) so that either all involved components are reporting the span data of a trace, or none.

> [!TIP]
> If you increase the sampling rate, you send more data your tracing backend and cause much higher network utilization in the cluster.
> To reduce costs and performance impacts in a production setup, a very low percentage of around 5% is recommended.

To configure an "always-on" sampling, set the sampling rate to 100%:

```yaml
apiVersion: telemetry.istio.io/v1
kind: Telemetry
metadata:
  name: mesh-default
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
apiVersion: telemetry.istio.io/v1
kind: Telemetry
metadata:
  name: tracing
  namespace: my-namespace
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: "my-app"
  tracing:
  - providers:
    - name: "kyma-traces"
    randomSamplingPercentage: 100.00
```

#### **Trace Context Without Spans**

To enable the propagation of the [W3C Trace Context](https://www.w3.org/TR/trace-context/) only, without reporting any spans (so the actual tracing feature is disabled), you must enable the `kyma-traces` provider with a sampling rate of 0. With this configuration, you get the relevant trace context into the [access logs](https://kyma-project.io/#/istio/user/tutorials/01-45-enable-istio-access-logs) without any active trace reporting.

  ```yaml
  apiVersion: telemetry.istio.io/v1
  kind: Telemetry
  metadata:
    name: mesh-default
    namespace: istio-system
  spec:
    tracing:
    - providers:
      - name: "kyma-traces"
      randomSamplingPercentage: 0
  ```

 <!-- tabs:end -->

### Eventing

The [Eventing](https://kyma-project.io/#/eventing-manager/user/README) module uses the [CloudEvents](https://cloudevents.io/) protocol (which natively supports the [W3C Trace Context](https://www.w3.org/TR/trace-context) propagation). Because of that, it propagates trace context properly. However, it doesn't enrich a trace with more advanced span data.

### Serverless

By default, all engines for the [Serverless](https://kyma-project.io/#/serverless-manager/user/README) module integrate the [Open Telemetry SDK](https://opentelemetry.io/docs/reference/specification/metrics/sdk/). Thus, the used middlewares are configured to automatically propagate the trace context for chained calls.

Because the Telemetry endpoints are configured by default, Serverless also reports custom spans for incoming and outgoing requests. You can [customize Function traces](https://kyma-project.io/#/serverless-manager/user/tutorials/01-100-customize-function-traces) to add more spans as part of your Serverless source code.

## Limitations

- **Throughput**: Assuming an average span with 40 attributes with 64 characters, the maximum throughput is 4200 span/sec ~= 15.000.000 spans/hour. If this limit is exceeded, spans are refused. To increase the maximum throughput, manually scale out the gateway by increasing the number of replicas for the trace gateway. See [Module Configuration and Status](https://kyma-project.io/#/telemetry-manager/user/01-manager?id=module-configuration).
- **Unavailability of Output**: For up to 5 minutes, a retry for data is attempted when the destination is unavailable. After that, data is dropped.
- **No Guaranteed Delivery**: The used buffers are volatile. If the OTel Collector instance crashes, trace data can be lost.
- **Multiple TracePipeline Support**: The maximum amount of TracePipeline resources is 5.
- **System Span Filtering**: System-related spans reported by Istio are filtered out without the opt-out option, for example:
  - Any communication of applications to the Telemetry gateways
  - Any communication from the gateways to backends

## Troubleshooting and Operations

Operational remarks can be found at [Operations](./../pipelines/operations.md).

For typical pipeline troubleshooting please see [Troubleshooting](./../pipelines/troubleshooting.md).

### Custom Spans Don’t Arrive at the Backend, but Istio Spans Do

**Cause**: Your SDK version is incompatible with the OTel Collector version.

**Solution**:

1. Check which SDK version you are using for instrumentation.
2. Investigate whether it is compatible with the OTel Collector version.
3. If required, upgrade to a supported SDK version.

### Trace Backend Shows Fewer Traces than Expected

**Cause**: By [default](#istio), only 1% of the requests are sent to the trace backend for trace recording.

**Solution**:

To see more traces in the trace backend, increase the percentage of requests by changing the default settings.
If you just want to see traces for one particular request, you can manually force sampling:

1. Create a `values.yaml` file.
   The following example sets the value to `60`, which means 60% of the requests are sent to the tracing backend.

```yaml
  apiVersion: telemetry.istio.io/v1
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

2. To override the default percentage, change the value for the **randomSamplingPercentage** attribute.
3. Deploy the `values.yaml` to your existing Kyma installation.
