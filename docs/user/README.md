# Telemetry Module

Learn more about the Telemetry Module. Use it to enable observability for your applications.

## What Is Telemetry?

Fundamentally, "Observability" is a measure of how well the application's external outputs can reflect the internal states of single components. The insights that an application and the surrounding infrastructure expose are displayed in the form of metrics, traces, and logs - collectively, that's called "telemetry" or ["signals"](https://opentelemetry.io/docs/concepts/signals/). These can be exposed by employing modern instrumentation.

![Stages of Observability](./assets/telemetry-stages.drawio.svg)

1. In order to implement Day-2 operations for a distributed application running in a container runtime, the single components of an application must expose these signals by employing modern instrumentation.
2. Furthermore, the signals must be collected and enriched with the infrastructural metadata in order to ship them to a target system.
3. Instead of providing a one-size-for-all backend solution, the Telemetry module supports you with instrumenting and shipping your telemetry data in a vendor-neutral way.
4. This way, you can conveniently enable observability for your application by integrating it into your existing or desired backends. Pick your favorite among many observability backends (available either as a service or as a self-manageable solution) that focus on different aspects and scenarios.

The Telemetry module focuses exactly on the aspects of instrumentation, collection, and shipment that happen in the runtime and explicitly defocuses on backends.

> [!TIP]
> An enterprise-grade setup demands a central solution outside the cluster, so we recommend in-cluster solutions only for testing purposes. If you want to install lightweight in-cluster backends for demo or development purposes, see [Integration Guides](#integration-guides).

## Features

To support telemetry for your applications, the Telemetry module provides the following features:

- Tooling for collection, filtering, and shipment: Based on the [Open Telemetry Collector](https://opentelemetry.io/docs/collector/), you can configure basic pipelines to collect, enrich, filter and ship telemetry data.
- Integration in a vendor-neutral way to a vendor-specific observability system: Based on the [OpenTelemetry protocol (OTLP)](https://opentelemetry.io/docs/reference/specification/protocol/), you can integrate backend systems.
- Guidance for the instrumentation: Based on [Open Telemetry](https://opentelemetry.io/), you get community samples on how to instrument your code using the [Open Telemetry SDKs](https://opentelemetry.io/docs/instrumentation/) in nearly every programming language.
- Enriching telemetry data by automatically adding metadata attributes (OTel resource attributes). This is done in compliance with established semantic conventions of OTel, ensuring that the enriched data adheres to industry best practices and is more meaningful for analysis. For details, see [Data Enrichment](./pipelines/enrichment.md).
- Opt-out of features for advanced scenarios: At any time, you can opt out for each data type, and use custom tooling to collect and ship the telemetry data.
- SAP BTP as first-class integration: Integration into SAP BTP Observability services, such as SAP Cloud Logging, is prioritized. For more information, see [Integrate with SAP Cloud Logging](integration/sap-cloud-logging/README.md).

The Telemetry API provides a hardened setup of an OTel Collector and also abstracts the underlying OTel Collector concept. Such abstraction has the following benefits:

- Compatibility: An abstraction layer supports compatibility when underlying features change.
- Migratability: Smooth migration experiences when switching underlying technologies or architectures.
- Native Kubernetes support: API provided by Kyma Telemetry supports an easy integration with Secrets, for example, served by the [SAP BTP Service Operator](https://github.com/SAP/sap-btp-service-operator#readme). Telemetry Manager takes care of the full lifecycle.
- Focus: The user doesn't need to understand the underlying concepts.

## Scope

The Telemetry module focuses only on the signals of application logs, distributed traces, and metrics. Other kinds of signals are not considered. Also, audit logs are not in scope.

Supported integration scenarios are neutral to the vendor of the target system.

## Architecture

The module consists of a manager component, which continuosly watches the API resources (pipelines) provided by the user and deploys sets of OTel Collectors doing. For more information, see [Architecture](architecture.md).

## Pipeline API

The API of the module consists of a Pipeline API available for every signal type. The three APIs have the same structure and base functionality which is explained in more detail in [Pipelines](pipelines/README.md).

For every pipeline kind, the signal specifics are described in more detail at:

- [Logs](./logs/README.md)
- [Traces](./traces/README.md)
- [Metrics](./metrics/README.md)

## Integration Guides

To learn about integration with SAP Cloud Logging, read [Integrate with SAP Cloud Logging](./integration/sap-cloud-logging/README.md). <!--- replace with Help Portal link once published? --->

For integration with other backends, such as Dynatrace, see:

- [Dynatrace](./integration/dynatrace/README.md)
- [Prometheus](./integration/prometheus/README.md)
- [Loki](./integration/loki/README.md)
- [Jaeger](./integration/jaeger/README.md)
- [Amazon CloudWatch and AWS X-Ray](./integration/aws-cloudwatch/README.md)

To learn how to collect data from applications based on the OpenTelemetry SDK, see:

- [OpenTelemetry Demo App](./integration/opentelemetry-demo/README.md)
- [Sample App](./integration/sample-app/)

## API / Custom Resource Definitions

The API of the Telemetry module is based on Kubernetes Custom Resource Definitions (CRD), which extend the Kubernetes API with custom additions. To inspect the specification of the Telemetry module API, see:

- [Telemetry CRD](./resources/01-telemetry.md)
- [LogPipeline CRD](./resources/02-logpipeline.md)
- [TracePipeline CRD](./resources/04-tracepipeline.md)
- [MetricPipeline CRD](./resources/05-metricpipeline.md)

## Resource Usage

To learn more about the resources used by the Telemetry module, see [Kyma Modules' Sizing](https://help.sap.com/docs/btp/sap-business-technology-platform/kyma-modules-sizing#telemetry).
