# Telemetry Module

Use the Telemetry module to collect telemetry signals (logs, traces, and metrics) from your applications and send them to your preferred observability backend.

## What Is Telemetry?

With telemetry signals, you can understand the behavior and health of your applications and infrastructure. The Telemetry module provides a standardized way to collect these signals and send them to your observability backend, where you can analyze them and troubleshoot issues.

The Telemetry module processes three types of signals:

- Logs: Time-stamped records of events that happen over time.
- Traces: The path of a request as it travels through your application's components.
- Metrics: Aggregated numerical data about the performance or state of a component over time.

![Stages of Observability](./assets/telemetry-stages.drawio.svg)

Telemetry signals flow through the following stages:

1. You instrument your application so that its components expose telemetry signals.
2. The signals are collected and enriched with infrastructural metadata.
3. You send the enriched signals to your preferred observability backend.
4. The backend stores your data, where you can analyze and visualize it.

The Telemetry module focuses on the collection, processing, and shipment stages of the observability workflow. It offers a vendor-neutral approach based on [OpenTelemetry](https://opentelemetry.io/) and doesn't force you into a specific backend. This means you can integrate with your existing observability platforms or choose from a wide range of available backends that best suit your operational needs.

> **Tip:**
> Build your first telemetry pipeline with the hands-on lesson [Collecting Application Logs and Shipping them to SAP Cloud Logging](https://learning.sap.com/learning-journeys/developing-applications-in-sap-btp-kyma-runtime/collecting-application-logs-and-shipping-to-sap-cloud-logging).

## Features

To support telemetry for your applications, the Telemetry module provides the following features:

- **Consistent Telemetry Pipeline API**: Use a streamlined set of APIs based on the [OTel Collector](https://opentelemetry.io/docs/collector/) to collect, filter, and ship your logs, metrics, and traces (see [Telemetry Pipeline API](pipelines.md)). You define a pipeline for each signal type to control how the data is processed and where it's sent. For details, see [Collecting Logs](./collecting-logs/README.md), [Collecting Traces](./collecting-traces/README.md), and [Collecting Metrics](./collecting-metrics/README.md).

- **Flexible Backend Integration**: The Telemetry module is optimized for integration with SAP BTP observability services, such as SAP Cloud Logging. You can also send data to any backend that supports the [OpenTelemetry protocol (OTLP)](https://opentelemetry.io/docs/specs/otel/protocol/), giving you the freedom to choose your preferred solution (see [Integrate With Your OTLP Backend](./integrate-otlp-backend/)).

  > **Recommendation:**
  > For production deployments, we recommend using a central telemetry solution located outside your cluster. For an example with SAP Cloud Logging, see [Integrate With SAP Cloud Logging](./integration/sap-cloud-logging/README.md).
  >
  > For testing or development, in-cluster solutions may be suitable. For examples such as Dynatrace (or to learn how to collect data from applications based on the OpenTelemetry Demo App), see [Integration Guides](https://kyma-project.io/#/telemetry-manager/user/integration/README).

- **Seamless Istio Integration**: The Telemetry module seamlessly integrates with the Istio module when both are present in your cluster. For details, see [Istio Integration](./architecture/istio-integration.md).

- **Automatic Data Enrichment**: The Telemetry module adds resource attributes as metadata, following OTel semantic conventions. This makes your data more consistent, meaningful, and ready for analysis in your observability backend. For details, see [Automatic Data Enrichment](./filter-and-process/automatic-data-enrichment.md).

- **Instrumentation Guidance**: To generate telemetry data, you must instrument your code. Based on [Open Telemetry](https://opentelemetry.io/) (OTel), you get community samples on how to instrument your code using the [Open Telemetry SDKs](https://opentelemetry.io/docs/languages/) in most programming languages.

- **Custom Tooling Support**: For advanced scenarios, you can opt out of the module's default collection and shipment mechanisms for individual data types. This enables you to use custom tooling to collect and ship the telemetry data.

## Scope

The Telemetry module focuses only on the signals of application logs, distributed traces, and metrics. Other kinds of signals are not considered. Also, audit logs are not in scope.

Supported integration scenarios are neutral to the vendor of the target system.

## Architecture

The Telemetry module is built around a central controller, Telemetry Manager, which dynamically configures and deploys data collection components based on your pipeline resources.

To understand how the core components interact, see [Architecture](architecture/README.md).

To learn how this model applies to each signal type, see:

- [Logs Architecture](./architecture/logs-architecture.md)
- [Traces Architecture](./architecture/traces-architecture.md)
- [Metrics Architecture](./architecture/metrics-architecture.md)

## API/Custom Resource Definitions

You configure the Telemetry module and its pipelines by creating and applying Kubernetes Custom Resource Definitions (CRDs), which extend the Kubernetes API with custom additions.

To understand and configure the module's global settings, refer to the [Telemetry CRD](./resources/01-telemetry.md).

To define how to collect, process, and ship a specific signal, use the pipeline CRDs:

- [LogPipeline CRD](./resources/02-logpipeline.md)
- [TracePipeline CRD](./resources/04-tracepipeline.md)
- [MetricPipeline CRD](./resources/05-metricpipeline.md)

## Resource Consumption

To learn more about the resources used by the Telemetry module, see [Kyma Modules' Sizing](https://help.sap.com/docs/btp/sap-business-technology-platform/kyma-modules-sizing#telemetry).
