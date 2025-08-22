# Traces Architecture

In the Kyma cluster, the Telemetry module provides a central deployment of an [OTel Collector](https://opentelemetry.io/docs/collector/) acting as a gateway. The gateway exposes endpoints to which all Kyma modules and users’ applications should send the trace data.

![Architecture](./../assets/traces-arch.drawio.svg)

1. An end-to-end request is triggered and populated across the distributed application. Every involved component propagates the trace context using the [W3C Trace Context](https://www.w3.org/TR/trace-context/) protocol.
2. After contributing a new span to the trace, the involved components send the related span data to the trace gateway using the `telemetry-otlp-traces` service. The communication happens based on the [OpenTelemetry Protocol (OTLP)](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/otlp.md) either using GRPC or HTTP.
3. Istio sends the related span data to the trace gateway as well.
4. The trace gateway discovers metadata that's typical for sources running on Kubernetes, like Pod identifiers, and then enriches the span data with that metadata.
5. Telemetry Manager configures the gateway according to the `TracePipeline` resource, including the target backend for the trace gateway. Also, it observes the trace flow to the backend and reports problems in the `TracePipeline` status.
6. The trace gateway sends the data to the observability system that's specified in your `TracePipeline` resource - either within the Kyma cluster, or, if authentication is set up, to an external observability backend.
7. You can analyze the trace data with your preferred backend system.

## Telemetry Manager

The TracePipeline resource is watched by Telemetry Manager, which is responsible for generating the custom parts of the OTel Collector configuration.

![Manager resources](./../assets/traces-resources.drawio.svg)

1. Telemetry Manager watches all TracePipeline resources and related Secrets.
2. Furthermore, Telemetry Manager takes care of the full lifecycle of the OTel Collector Deployment itself. Only if you defined a TracePipeline, the collector is deployed.
3. Whenever the configuration changes, it validates the configuration and generates a new configuration for OTel Collector, where a ConfigMap for the configuration is generated.
4. Referenced Secrets are copied into one Secret that is mounted to the OTel Collector as well.

## Trace Gateway

In a Kyma cluster, the trace gateway is the central component to which all components can send their individual spans. The gateway collects, enriches, and dispatches the data to the configured backend. For more information, see [otlp-input](./../pipelines/otlp-input.md).
