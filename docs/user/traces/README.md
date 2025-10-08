# Collecting Traces

With the Telemetry module, you can collect distributed traces to understand the flow of requests through your applications and infrastructure. To begin collecting traces, you create a `TracePipeline` resource. It automatically collects OTLP traces and can be configured to collect traces from the Istio service mesh.

## Overview

The TracePipeline is a Kubernetes Custom Resource (CR) that configures trace collection for your cluster. When you create a TracePipeline, it automatically provisions a trace gateway that provides a central OTLP endpoint receiving traces pushed from applications (for details, see [Tracing Architecture](architecture.md)).

The pipeline enriches all collected traces with Kubernetes metadata before sending them to your chosen backend.

The Traces feature is optional. If you don't create a `TracePipeline`, the trace gateway is not deployed.

## Prerequisites

For the recording of a distributed trace, every involved component must propagate at least the trace context. For details, see [Trace Context](https://www.w3.org/TR/trace-context/#problem-statement).

- In Kyma, all modules involved in users’ requests support the [W3C Trace Context](https://www.w3.org/TR/trace-context) protocol. The involved Kyma modules are, for example, Istio, Serverless, and Eventing.
- Your application also must propagate the W3C Trace Context for any user-related activity. This can be achieved easily using the [Open Telemetry SDKs](https://opentelemetry.io/docs/instrumentation/) available for all common programming languages. If your application follows that guidance and is part of the Istio Service Mesh, it’s already outlined with dedicated span data in the trace data collected by the Kyma telemetry setup.
- Furthermore, your application must enrich a trace with additional span data and send these data to the cluster-central telemetry services. You can achieve this with [Open Telemetry SDKs](https://opentelemetry.io/docs/instrumentation/).

With the default configuration, the trace gateway collects push-based OTLP traces of any container running in the Kyma runtime, and the data is shipped to your backend.

## Minimal TracePipeline

For a minimal setup, you only need to create a TracePipeline that specifies your backend destination (see [Integrate With Your OTLP Backend](./../pipelines/otlp-output.md)).

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

By default, this minimal pipeline collects push-based OTLP traces of any container running in the Kyma runtime.

It activates cluster-internal endpoints to receive traces in the OTLP format. Applications can push traces directly to these URLs:

- GRPC: `http://telemetry-otlp-traces.kyma-system:4317`
- HTTP: `http://telemetry-otlp-traces.kyma-system:4318`

## Configure Trace Collection

You can adjust the TracePipeline using runtime configuration with the available parameters and attributes (see [TracePipeline: Custom Resource Parameters](https://kyma-project.io/#/telemetry-manager/user/resources/04-tracepipeline?id=custom-resource-parameters)).

- If you use Istio, activate Istio tracing. For details, see [Configure Istio Tracing](istio-support.md). You can adjust which percentage of the trace data is collected.
- The Serverless module integrates the [OpenTelemetry SDK](https://opentelemetry.io/docs/specs/otel/metrics/sdk/) by default. It automatically propagates the trace context for chained calls and reports custom spans for incoming and outgoing requests. You can add more spans within your Function's source code. For details, see [Customize Function Traces](https://kyma-project.io/#/serverless-manager/user/tutorials/01-100-customize-function-traces).
- The Eventing module uses the CloudEvents protocol, which natively supports [W3C Trace Context](https://www.w3.org/TR/trace-context/) propagation. It ensures the trace context is passed along but doesn't enrich a trace with more advanced span data.

## Limitations

- **Throughput**: Assuming an average span with 40 attributes with 64 characters, the maximum throughput is 4200 span/sec ~= 15.000.000 spans/hour. If this limit is exceeded, spans are refused. To increase the maximum throughput, manually scale out the gateway by increasing the number of replicas for the trace gateway. See [Module Configuration and Status](https://kyma-project.io/#/telemetry-manager/user/01-manager?id=module-configuration).
- **Unavailability of Output**: For up to 5 minutes, a retry for data is attempted when the destination is unavailable. After that, data is dropped.
- **No Guaranteed Delivery**: The used buffers are volatile. If the OTel Collector instance crashes, trace data can be lost.
- **Multiple TracePipeline Support**: The maximum amount of TracePipeline resources is 5.
- **System Span Filtering**: System-related spans reported by Istio are filtered out without the opt-out option, for example:
  - Any communication of applications to the Telemetry gateways
  - Any communication from the gateways to backends

## Troubleshooting and Operations

Operational remarks can be found at [Telemetry Pipeline Operations](./../pipelines/operations.md).

For typical pipeline troubleshooting please see [Telemetry Pipeline Troubleshooting](./../pipelines/troubleshooting.md).

### Custom Spans Don’t Arrive at the Backend, but Istio Spans Do

**Cause**: Your SDK version is incompatible with the OTel Collector version.

**Solution**:

1. Check which SDK version you are using for instrumentation.
2. Investigate whether it is compatible with the OTel Collector version.
3. If required, upgrade to a supported SDK version.

### Trace Backend Shows Fewer Traces than Expected

**Cause**: By default, only 1% of the requests are sent to the trace backend for trace recording, see [Traces Istio Support](./istio-support.md).

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
