# Set Up the OTLP Input

Use the default OTLP input to collect telemetry data from your instrumented applications and customize how that data is processed by the pipeline. You can specify which input data goes to which backend. If you're using Istio, data is collected from the service mesh.

## Overview

When you create any LogPipeline, TracePipeline, or MetricPipeline, the Telemetry module automatically deploys the respective gateway. This opens a stable, cluster-internal [OTLP](https://opentelemetry.io/docs/specs/otel/protocol/) endpoint for each signal type, ready to receive data from your applications.

Each endpoint listens on port `4317` for gRPC (default) and on port `4318` for HTTP.

![OTLP Input](./assets/otlp-input.drawio.svg)

## Configure Your Application's OTLP Exporter

To send data from your application, first instrument your code using an [OTel SDK](https://opentelemetry.io/docs/languages/) for your programming language. The SDK's OTLP exporter sends the collected telemetry data to a backend.

It's recommended that you configure the exporter's destination by setting the standard [environment variables](https://opentelemetry.io/docs/languages/sdk-configuration/otlp-exporter/#otel_exporter_otlp_traces_endpoint) in your application's deployment. This method avoids hardcoding endpoints in your application code.

Use the following environment variables to set the OTLP endpoint for each signal type:

- Traces gRPC: `export OTEL_EXPORTER_OTLP_TRACES_ENDPOINT="http://telemetry-otlp-traces.kyma-system:4317"`
- Traces HTTP: `export OTEL_EXPORTER_OTLP_TRACES_ENDPOINT="http://telemetry-otlp-traces.kyma-system:4318/v1/traces"`
- Metrics gRPC: `export OTEL_EXPORTER_OTLP_METRICS_ENDPOINT="http://telemetry-otlp-metrics.kyma-system:4317"`
- Metrics HTTP: `export OTEL_EXPORTER_OTLP_METRICS_ENDPOINT="http://telemetry-otlp-metrics.kyma-system:4318/v1/metrics"`
- Logs gRPC: `export OTEL_EXPORTER_OTLP_LOGS_ENDPOINT="http://telemetry-otlp-logs.kyma-system:4317"`
- Logs HTTP: `export OTEL_EXPORTER_OTLP_LOGS_ENDPOINT="http://telemetry-otlp-logs.kyma-system:4318/v1/logs"`

> [!NOTE]
> If your cluster uses Istio, communication with these endpoints is automatically secured with mTLS. For details, see [Istio Integration](./architecture/istio-integration.md).

## Verify the Endpoints

To see whether you've set up your gateways and their push endpoints successfully, check the status of the default `Telemetry` resource:

```sh
kubectl -n kyma-system get telemetries.operator.kyma-project.io default -oyaml
```

The output shows the available endpoints and the pipeline health under the status.endpoints section:

```yaml
  endpoints:
    metrics:
      grpc: http://telemetry-otlp-metrics.kyma-system:4317
      http: http://telemetry-otlp-metrics.kyma-system:4318
    traces:
      grpc: http://telemetry-otlp-traces.kyma-system:4317
      http: http://telemetry-otlp-traces.kyma-system:4318
    logs:
      grpc: http://telemetry-otlp-logs.kyma-system:4317
      http: http://telemetry-otlp-logs.kyma-system:4318
```

## Route Specific Inputs to Different Backends

For logs and metrics: If you have multiple pipelines sending data to different backends, you can specify which inputs are active for each pipeline. This is useful if you want one pipeline to handle only OTLP data and another to handle only data from a different source.

> [!TIP]
> For more granular control, you can also filter incoming OTLP data by namespace. For details, see [Filter Logs](./filter-and-process/filter-logs.md) and [Filter Metrics](./filter-and-process/filter-metrics.md).

For example, if you want to analyze **otlp** input data in one backend and only data from the log-specific **runtime** input in another backend, then disable the **otlp** input for the second backend. By default, **otlp** input is enabled.

```yaml
...
  input:
    runtime:
      enabled: true
    otlp:
      enabled: false
```
