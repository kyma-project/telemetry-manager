# Logs

With application logs, you can debug an application and derive the internal state of an application. The Telemetry module supports observing your applications with logs of the correct severity level and context.

## Overview

The Telemetry module provides an API which configures a log gateway for push-based collection of logs using OTLP and, optionally, an agent for the collection of logs of any container printing logs to the `stdout/stderr` channel running in the Kyma runtime. Kyma modules like [Istio](https://kyma-project.io/#/istio/user/README) contribute access logs. The Telemetry module enriches the data and ships them to your chosen backend (see [Vendors who natively support OpenTelemetry](https://opentelemetry.io/ecosystem/vendors/)).

You can configure the log gateway and agent with external systems using runtime configuration with a dedicated Kubernetes API ([CRD](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions)) named LogPipeline. A LogPipeline is following the structure and characteristic of a Telemetry [pipeline](./../pipelines/README.md) and offers these collection features:

- [`otlp` input](./../pipelines/otlp-input.md): Ingest OTLP logs via the push endpoints.
- [`application` input](./application-input.md): Activate collection of application logs from `stdout/stderr` channels of any container in the cluster using the [`application` input](./application-input.md)
- Activate Istio access logs. For details, see [Istio](#istio).

For an example, see [Sample LogPipeline](sample.md), check out the available parameters and attributes under [LogPipeline](./../resources/02-logpipeline.md) and checkout the involved components of a LogPipeline at the [Architecture](architecture.md).

The Log feature is optional. If you don’t want to use it, simply don’t set up a LogPipeline.

## Prerequisites

- Before you can collect logs from a component, it must emit the logs. Typically, it uses a logger framework for the used language runtime (like Node.js) and prints them to the `stdout` or `stderr` channel ([Kubernetes: How nodes handle container logs](https://kubernetes.io/docs/concepts/cluster-administration/logging/#how-nodes-handle-container-logs)). Alternatively, you can use the [OTel SDK](https://opentelemetry.io/docs/languages/) to use the [push-based OTLP format](https://opentelemetry.io/docs/specs/otlp/).

- If you want to emit the logs to the `stdout/stderr` channel, use structured logs in a JSON format with a logger library like log4J. With that, the log agent can parse your log and enrich all JSON attributes as log attributes, and a backend can use that.

- If you prefer the push-based alternative with OTLP, also use a logger library like log4J. However, you additionally instrument that logger and bridge it to the OTel SDK. For details, see [OpenTelemetry: New First-Party Application Logs](https://opentelemetry.io/docs/specs/otel/logs/#new-first-party-application-logs).

## Basic Pipeline

The minimal pipeline will define an [`otlp` output](./../pipelines/otlp-output.md) and have the [`otlp` input](./../pipelines/otlp-input.md) enabled by default.

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: LogPipeline
metadata:
  name: backend
output:
    otlp:
      endpoint:
        value: http://myEndpoint:4317
```

This example will accept traffic from the OTLP log endpoint, filtered by system namespaces, and will forward them to the configured backend. The following push URLs are set up:

- GRPC: `http://telemetry-otlp-logs.kyma-system:4317`
- HTTP: `http://telemetry-otlp-logs.kyma-system:4318`

The default protocol for shipping the data to a backend is GRPC, but you can choose HTTP instead. Ensure that the correct port is configured as part of the endpoint.

## Application Input

The most common input used on a LogPipline is the [`application` input](./application-input.md). It will enable the collection of application logs from all containers tailing the log files of the container runtime. The most basic example which is enabling collection on all namespaces except the system namespaces:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: LogPipeline
metadata:
  name: backend
input:
  application:
    enabled: true
output:
    otlp:
      endpoint:
        value: http://myEndpoint:4317
```

For more details, please see [`application` input](./application-input.md).

## Kyma Modules With Logging Capabilities

Kyma bundles modules that can be involved in user flows. If you want to collect all logs of all modules, enable the [`application` input](./application-input.md) for the `kyma-system` namespace.

### Istio

The Istio module is crucial as it provides the [Ingress Gateway](https://istio.io/latest/docs/tasks/traffic-management/ingress/ingress-control/). Typically, this is where external requests enter the cluster scope. Furthermore, every component that’s part of the Istio Service Mesh runs an Istio proxy. Using the Istio telemetry API, you can enable access logs for the Ingress Gateway and the proxies individually.

The following example configures all Istio proxies with the `kyma-logs` extension provider, which, by default, reports access logs to the log gateway of the Telemetry module.

```yaml
apiVersion: telemetry.istio.io/v1
kind: Telemetry
metadata:
  name: mesh-default
  namespace: istio-system
spec:
  accessLogging:
    - providers:
        - name: kyma-logs
```

The Kyma Istio module provides a second extension provider `stdout-json` not based on OTLP. Please assure that you always use the `kyma-logs` provider as only this will work with the OTLP based LogPipeline output.
More details and configuration options can be found at [Configure Istio Access Logs](./istio.md).

## Limitations

- **Throughput**:
  - When pushing OTLP logs of an average size of 2KB to the log gateway, using its default configuration (two instances), the Telemetry module can process approximately 12,000 logs per second (LPS). To ensure availability, the log gateway runs with multiple instances. For higher throughput, manually scale out the gateway by increasing the number of replicas. See [Module Configuration and Status](https://kyma-project.io/#/telemetry-manager/user/01-manager?id=module-configuration). Ensure that the chosen scaling factor does not exceed the maximum throughput of the backend, as it may refuse logs if the rate is too high.
  - For example, to scale out the gateway for scenarios like a `Large` instance of SAP Cloud Logging (up to 30,000 LPS), you can raise the throughput to about 20,000 LPS by increasing the number of replicas to 4 instances.
  - The log agent, running one instance per node, handles tailing logs from stdout using the `runtime` input. When writing logs of an average size of 2KB to stdout, a single log agent instance can process approximately 9,000 LPS.
- **Load Balancing With Istio**: By design, the connections to the gateway are long-living connections (because OTLP is based on gRPC and HTTP/2). For optimal scaling of the gateway, the clients or applications must balance the connections across the available instances, which is automatically achieved if you use an Istio sidecar. If your application has no Istio sidecar, the data is always sent to one instance of the gateway.
- **Unavailability of Output**: For up to 5 minutes, a retry for data is attempted when the destination is unavailable. After that, data is dropped.
- **No Guaranteed Delivery**: The used buffers are volatile. If the gateway or agent instances crash, logs data can be lost.
- **Multiple LogPipeline Support**: The maximum amount of LogPipeline resources is 5.

## Troubleshooting and Operations

Operational remarks can be found at [Operations](./../pipelines/operations.md).
There are no signal specific routines defined, for typical pipeline troubleshooting please see [Troubleshooting](./../pipelines/troubleshooting.md).
