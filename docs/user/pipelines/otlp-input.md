OTLP Input


The `otlp` input of a pipeline is enabled by default and will enable a cluster interal endpoint accepting OTLP data.
To see whether you've set up your push endpoints successfully, check the status of the default Telemetry resource:

```sh
kubectl -n kyma-system get telemetries.operator.kyma-project.io default -oyaml
```

In the status of the returned resource, you see the pipeline health as well as the available push endpoints:

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

For every signal type, there's a dedicated endpoint to which you can push data using [OTLP](https://opentelemetry.io/docs/specs/otel/protocol/). OTLP supports GRPC and HTTP-based communication, each having its individual port on every endpoint. Use port `4317` for GRPC and `4318` for HTTP.

![Gateways-Plain](assets/gateways-plain.drawio.svg)

Applications that support OTLP typically use the [OTel SDK](https://opentelemetry.io/docs/languages/) for instrumentation of the data. You can either configure the endpoints hardcoded in the SDK setup, or you use standard [environment variables](https://opentelemetry.io/docs/languages/sdk-configuration/otlp-exporter/#otel_exporter_otlp_traces_endpoint) configuring the OTel exporter, for example:

- Traces GRPC: `export OTEL_EXPORTER_OTLP_TRACES_ENDPOINT="http://telemetry-otlp-traces.kyma-system:4317"`
- Traces HTTP: `export OTEL_EXPORTER_OTLP_TRACES_ENDPOINT="http://telemetry-otlp-traces.kyma-system:4318/v1/traces"`
- Metrics GRPC: `export OTEL_EXPORTER_OTLP_METRICS_ENDPOINT="http://telemetry-otlp-metrics.kyma-system:4317"`
- Metrics HTTP: `export OTEL_EXPORTER_OTLP_METRICS_ENDPOINT="http://telemetry-otlp-metrics.kyma-system:4318/v1/metrics"`

The Telemetry module automatically detects whether the Istio module is added to your cluster, and injects Istio sidecars to the Telemetry components. Additionally, the ingestion endpoints of gateways are configured to allow traffic in the permissive mode, so they accept mTLS-based communication as well as plain text.

![Gateways-Istio](assets/gateways-istio.drawio.svg)

Clients in the Istio service mesh transparently communicate to the gateway with mTLS. Clients that don't use Istio can communicate with the gateway in plain text mode. The same pattern applies for the communication to the backends running in the cluster. External clusters use the configuration as specified in the pipelines output section.