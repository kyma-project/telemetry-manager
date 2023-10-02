
# Kyma Telemetry Module

## What is Telemetry?

Fundamentally, ["Observability"](https://opentelemetry.io/docs/concepts/observability-primer/) is a measure of how well the application's external outputs can reflect the internal states of single components. The insights that an application and the surrounding infrastructure expose are displayed in the form of metrics, traces, and logs - collectively, that's called "telemetry" or ["signals"](https://opentelemetry.io/docs/concepts/signals/). These can be exposed by employing modern instrumentation.

![Stages of Observability](./assets/telemetry-stages.drawio.svg)

1. In order to implement Day-2 operations for a distributed application running in a container runtime, the single components of an application must expose these signals by employing modern instrumentation.
2. Furthermore, the signals must be collected and enriched with the infrastructural metadata in order to ship them to a target system.
3. Instead of providing a one-size-for-all backend solution, Kyma supports you with instrumenting and shipping your telemetry data in a vendor-neutral way.
4. This way, you can conveniently enable observability for your application by integrating it into your existing or desired backends. Pick your favorite among many observability backends, available either as a service or as a self-manageable solution, that focus on different aspects and scenarios.

Kyma's Telemetry module focuses exactly on the aspects of instrumentation, collection, and shipment that happen in the runtime and explicitly defocuses on backends.

> **TIP:** An enterprise-grade setup demands a central solution outside the cluster, so we recommend in-cluster solutions only for testing purposes. If you want to install lightweight in-cluster backends for demo or development purposes, check the [Telemetry tutorials](05-tutorials.md).

## Features

To support telemetry for your applications, Kyma's Telemetry module provides the following features:

- Tooling for collection, filtering, and shipment: Based on the [Open Telemetry Collector](https://opentelemetry.io/docs/collector/) and [Fluent Bit](https://fluentbit.io/), you can configure basic pipelines to filter and ship telemetry data.
- Integration in a vendor-neutral way to a vendor-specific observability system (traces and metrics only): Based on the [OpenTelemetry protocol (OTLP)](https://opentelemetry.io/docs/reference/specification/protocol/), you can integrate backend systems.
- Guidance for the instrumentation (traces and metrics only): Based on [Open Telemetry](https://opentelemetry.io/), you get community samples on how to instrument your code using the [Open Telemetry SDKs](https://opentelemetry.io/docs/instrumentation/) in nearly every programming language.
- Opt-out from features for advanced scenarios: At any time, you can opt out for each data type, and use custom tooling to collect and ship the telemetry data.
- SAP BTP as first-class integration: Integration into BTP Observability services is prioritized.

## Scope

The Telemetry module focuses only on the signals of application logs, distributed traces, and metrics. Other kinds of signals are not considered. Also, logs like audit logs or operational logs are not in scope.

Supported integration scenarios are neutral to the vendor of the target system.

Currently, logs are based on the Fluent Bit protocol. If you're curious about the progress to switch to the vendor-neutral OTLP protocol, follow this [epic](https://github.com/kyma-project/kyma/issues/16307).

## Components

![Components](./assets/telemetry-components.drawio.svg)

### Telemetry Manager

Kyma's Telemetry module ships Telemetry Manager as its core component. Telemetry Manager is a Kubernetes [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) that implements the Kubernetes controller pattern and manages the whole lifecycle of all other components covered in the Telemetry module. Telemetry Manager watches for the user-created Kubernetes resources: LogPipeline, TracePipeline, and, in the future, MetricPipeline. In these resources, you specify what data of a signal type to collect and where to ship it.
If Telemetry Manager detects a configuration, it rolls out the relevant components on demand.

For more information, see [Telemetry Manager](01-manager.md).

### Log Agent

The log agent is based on a [Fluent Bit](https://fluentbit.io/) installation running as a [DaemonSet](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/). It reads all containers' logs in the runtime and ships them according to a LogPipeline configuration.

For more information, see [Logs](02-logs.md).

### Trace Gateway

The trace gateway is based on an [OTel Collector](https://opentelemetry.io/docs/collector/) [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/). It provides an [OTLP-based](https://opentelemetry.io/docs/reference/specification/protocol/) endpoint to which applications can push the trace signals. According to a TracePipeline configuration, the gateway processes and ships the trace data to a target system.

For details, see [Traces](03-traces.md).

### Metric Gateway/Agent

> **NOTE:** The feature is not available yet. To understand the current progress, watch this [epic](https://github.com/kyma-project/kyma/issues/13079).

The metric gateway and agent are based on an [OTel Collector](https://opentelemetry.io/docs/collector/) [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/) and a [DaemonSet](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/). The gateway provides an [OTLP-based](https://opentelemetry.io/docs/reference/specification/protocol/) endpoint to which applications can push the metric signals. The agent scrapes annotated Prometheus-based workloads. According to a MetricPipeline configuration, the gateway processes and ships the metric data to a target system.

For more information, see [Metrics](04-metrics.md).

## API / Custom Resource Definitions

- [Telemetry](resources/01-telemetry.md)
- [LogPipeline](resources/02-logpipeline.md)
- [LogParser](resources/03-logparser.md)
- [TracePipeline](resources/04-tracepipeline.md)
- [MetricPipeline](resources/05-metricpipeline.md)
