# Metrics Architecture

In the Telemetry module, a central in-cluster Deployment of an [OTel Collector](https://opentelemetry.io/docs/collector/) acts as a gateway. The gateway exposes endpoints for the [OpenTelemetry Protocol (OTLP)](https://opentelemetry.io/docs/specs/otlp/) for GRPC and HTTP-based communication using the dedicated `telemetry-otlp-metrics` service, to which all Kyma modules and users’ applications send the metrics data.

Optionally, the Telemetry module provides a DaemonSet of an OTel Collector acting as an agent. This agent can pull metrics of a workload and the Istio sidecar in the [Prometheus pull-based format](https://prometheus.io/docs/instrumenting/exposition_formats) and can provide runtime-specific metrics for the workload.

![Architecture](./../assets/metrics-arch.drawio.svg)

1. An application (exposing metrics in OTLP) sends metrics to the central metric gateway service.
2. An application (exposing metrics in Prometheus protocol) activates the agent to scrape the metrics with an annotation-based configuration.
3. Additionally, you can activate the agent to pull metrics of each Istio sidecar.
4. The agent supports collecting metrics from the Kubelet and Kubernetes APIServer.
5. The agent converts and sends all collected metric data to the gateway in OTLP.
6. The gateway discovers the metadata and enriches all received data with typical metadata of the source by communicating with the Kubernetes APIServer. Furthermore, it filters data according to the pipeline configuration.
7. Telemetry Manager configures the agent and gateway according to the `MetricPipeline` resource specification, including the target backend for the metric gateway. Also, it observes the metrics flow to the backend and reports problems in the MetricPipeline status.
8. The metric gateway sends the data to the observability system that's specified in your `MetricPipeline` resource - either within the Kyma cluster, or, if authentication is set up, to an external observability backend.
9. You can analyze the metric data with your preferred backend system.

## Telemetry Manager

The MetricPipeline resource is watched by Telemetry Manager, which is responsible for generating the custom parts of the OTel Collector configuration.

![Manager resources](./../assets/metrics-resources.drawio.svg)

1. Telemetry Manager watches all MetricPipeline resources and related Secrets.
2. Furthermore, Telemetry Manager takes care of the full lifecycle of the gateway Deployment and the agent DaemonSet. Only if you defined a MetricPipeline, the gateway and agent are deployed.
3. Whenever the user configuration changes, Telemetry Manager validates it and generates a single configuration for the gateway and agent.
4. Referenced Secrets are copied into one Secret that is mounted to the gateway as well.

## Metric Gateway

In a Kyma cluster, the metric gateway is the central component to which all applications can send their individual metrics. The gateway collects, enriches, and dispatches the data to the configured backend. For more information, see [otlp-input](./../pipelines/otlp-input.md).

## Metric Agent

If a MetricPipeline configures a feature in the `input` section, an additional DaemonSet is deployed acting as an agent. The agent is also based on an [OTel Collector](https://opentelemetry.io/docs/collector/) and encompasses the collection and conversion of Prometheus-based metrics. Hereby, the workload puts a `prometheus.io/scrape` annotation on the specification of the Pod or service, and the agent collects it. The agent sends all data in OTLP to the central gateway.
