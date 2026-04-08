# Architecture

The Telemetry module consists of a manager component, which continuosly watches the user-provided pipeline resources and deploys the respective OTel Collectors. Learn more about the architecture and how the components interact.

## Overview

The Telemetry API provides a robust, pre-configured OpenTelemetry (OTel) Collector setup that abstracts its underlying complexities. This approach delivers several key benefits:

- Compatibility: Maintains stability and functionality even as underlying OTel Collector features evolve, reducing the need for constant updates on your end.
- Migratability: Facilitates smooth transitions when you switch underlying technologies or architectures.
- Native Kubernetes Support: Offers seamless integration with Secrets, for example, served by the SAP BTP Service Operator, and the Telemetry Manager automatically handles the full lifecycle of all components.
- Focus: Reduces the need to understand intricate underlying OTel Collector concepts, allowing you to focus on your application development.

![Components](./../assets/telemetry-arch.drawio.svg)

## Telemetry Manager

Telemetry Manager, the core component of the module, is a Kubernetes [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) that implements the Kubernetes controller pattern and manages the whole lifecycle of all other Telemetry components. It performs the following tasks:

1. Watch the module configuration for changes and sync the module status to it.
2. Watch the user-created Kubernetes resources LogPipeline, TracePipeline, and MetricPipeline. In these resources, you specify what data of a signal type to collect and where to ship it.
3. Manage the lifecycle of the self monitor, the OTLP Gateway, and the signal-specific agents (that is, the Log Agent and Metric Agent).
   The OTLP Gateway is deployed when you create any pipeline resource (LogPipeline, TracePipeline, or MetricPipeline).

![Manager](./../assets/manager-resources.drawio.svg)

## OTLP Gateway and Agents

The OTLP Gateway and signal-specific agents (that is, the Log Agent and Metric Agent) handle the incoming telemetry data. Telemetry Manager deploys them based on your pipeline configuration.

The OTLP Gateway is based on an [OTel Collector](https://opentelemetry.io/docs/collector/) [DaemonSet](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/) running one instance per cluster node. It acts as the central endpoint to which your applications push telemetry data in the OTLP format. The gateway enriches and filters the data, and then dispatches it to the backends configured in your pipeline resources. The gateway handles all signal types (logs, traces, and metrics) in a single unified component.

Applications can send data to the gateway using these service endpoints:

- Logs/Metrics/Traces: `http://telemetry-otlp.kyma-system:4317` (gRPC) or `http://telemetry-otlp.kyma-system:4318` (HTTP) [Preferred]
- Logs: `http://telemetry-otlp-logs.kyma-system:4317` (gRPC) or `http://telemetry-otlp-logs.kyma-system:4318` (HTTP)
- Metrics: `http://telemetry-otlp-metrics.kyma-system:4317` (gRPC) or `http://telemetry-otlp-metrics.kyma-system:4318` (HTTP)
- Traces: `http://telemetry-otlp-traces.kyma-system:4317` (gRPC) or `http://telemetry-otlp-traces.kyma-system:4318` (HTTP)

Agents run as [DaemonSet](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/) and pull data from the respective node.

- **Log Agent**: Collects logs from the stdout/stderr output of all containers on a node. For details, see [Logs Architecture](./logs-architecture.md). As an alternative to the OTLP-based log feature, you can use a log agent based on a [Fluent Bit](https://fluentbit.io/) installation running as a DaemonSet. It reads all containers’ logs in the runtime and ships them according to your LogPipeline configuration. For details, see [Application Logs (Fluent Bit)](./../02-logs.md).
- **Metric Agent**: Scrapes Prometheus-annotated workloads on each node. For details, see [Metrics Architecture](./metrics-architecture.md).

## Self Monitor

The Telemetry module includes a [Prometheus](https://prometheus.io/)-based self-monitor that collects and evaluates health metrics from the OTLP Gateway and agents. Telemetry Manager uses this data to report the current health status in your pipeline resources.

You can also use these health metrics in your own observability backend to set up alerts and dashboards for your telemetry pipelines. For details, see [Monitor Pipeline Health](./../monitor-pipeline-health.md).

![Self-Monitor](./../assets/manager-arch.drawio.svg)
