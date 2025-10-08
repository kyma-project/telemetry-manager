# Metrics

With the Telemetry module, you can collect all relevant metrics of a workload in a Kyma cluster and ship them to a backend for further analysis.

## Overview

The Telemetry module provides an API which configures a metric gateway and, optionally, an agent for the collection and shipment of metrics of any container running in the Kyma runtime. Kyma modules like [Istio](https://kyma-project.io/#/istio/user/README) or [Serverless](https://kyma-project.io/#/serverless-manager/user/README) contribute metrics instantly, and the Telemetry module enriches the data. You can choose among multiple [vendors for OTLP-based backends](https://opentelemetry.io/ecosystem/vendors/).

You can configure the metric gateway with external systems using runtime configuration with a dedicated Kubernetes API ([CRD](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions)) named MetricPipeline. A MetricPipeline is following the structure and characteristic of a Telemetry [pipeline](./../pipelines/README.md) and offers these collection features:

- `otlp` input: Ingest OTLP metrics via the push endpoints, see [Telemetry Pipeline OTLP Input](./../pipelines/otlp-input.md).
- `prometheus` input: Requires annotating your Kubernetes Services (or Pods without sidecars and Services) to expose the metrics endpoint for scraping. You must set prometheus.io/scrape: "true" along with prometheus.io/port: "". You can use the annotations prometheus.io/path (defaults to /metrics), and prometheus.io/scheme (defaults to http unless an Istio sidecar is present with security.istio.io/tlsMode=istio and then https is used) to control specific details of the scraping process. See [Metrics Prometheus Input](./prometheus-input.md).
- `runtime` input: Enables collecting Kubernetes runtime metrics. You can configure which resource types (Pods, containers, Nodes, and so on) to include or exclude, see [Metrics Runtime Input](./runtime-input.md).
- `istio` input: Collects Istio metrics and, optionally, Envoy proxy metrics (if envoyMetrics.enabled: true), see [Metrics Istio Input](./istio-input.md).

For an example, see [Sample MetricPipeline](./sample.md) and check out the available parameters and attributes under [MetricPipeline](./../resources/05-metricpipeline.md) and checkout the involved components of a MetricPipeline at the [Metrics Architecture](./../architecture/metrics-architecture.md).

The Metric feature is optional. If you don't want to use it, simply don't set up a MetricPipeline.

## Prerequisites

- Before you can collect metrics data from a component, it must expose (or instrument) the metrics. Typically, it instruments specific metrics for the used language runtime (like Node.js) and custom metrics specific to the business logic. Also, the exposure can be in different formats, like the pull-based Prometheus format or the [push-based OTLP format](https://opentelemetry.io/docs/specs/otlp/).

- If you want to use Prometheus-based metrics, you must have instrumented your application using a library like the [Prometheus client library](https://prometheus.io/docs/instrumenting/clientlibs/), with a port in your workload exposed serving as a Prometheus metrics endpoint.

- For the instrumentation, you typically use an SDK, namely the [Prometheus client libraries](https://prometheus.io/docs/instrumenting/clientlibs/) or the [Open Telemetry SDKs](https://opentelemetry.io/docs/instrumentation/). Both libraries provide extensions to activate language-specific auto-instrumentation like for Node.js, and an API to implement custom instrumentation.

## Basic Pipeline

The minimal pipeline will define an `otlp` output and have the `otlp` input enabled by default, see [Telemetry Pipeline OTLP Output](./../pipelines/otlp-output.md) and [Telemetry Pipeline OTLP Input](./../pipelines/otlp-input.md).

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
output:
    otlp:
      endpoint:
        value: http://myEndpoint:4317
```

This example will accept traffic from the OTLP metric endpoint filtered by system namespaces and will forward them to the configured backend. The following push URLs are set up:

- GRPC: `http://telemetry-otlp-metrics.kyma-system:4317`
- HTTP: `http://telemetry-otlp-metrics.kyma-system:4318`

The default protocol for shipping the data to a backend is GRPC, but you can choose HTTP instead. Ensure that the correct port is configured as part of the endpoint.

## Prometheus Input

To start scraping metrics from applications being exposed in the Prometheus pull-based way, use the `prometheus` input. It will discover all endpoints on base of the `prometheus.io/scrape` annotation. The most basic example which is enabling collection on all namespaces except the system namespaces:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
input:
  prometheus:
    enabled: true
output:
    otlp:
      endpoint:
        value: http://myEndpoint:4317
```

For more details, please see [Metrics Prometheus Input](./prometheus-input.md).

## Telemetry Health Input

Every MetricPipeline will activate the telemetry `health` input by default. This input will collect metrics about the health of all telemetry pipelines. This input cannot be deactivated and has no dedicated API element.

For more details, please see [Metrics Health Input](./health-input.md).

## Runtime Input

To collect typical system metrics like CPU and memory usage about your containers, activate the `runtime` input. The most basic example which is enabling collection on all namespaces except the system namespaces:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
input:
  runtime:
    enabled: true
output:
    otlp:
      endpoint:
        value: http://myEndpoint:4317
```

For more details, please see [Metrics Runtime Input](./runtime-input.md).

## Istio Input

To collect metrics from all the Istio sidecars running withour applications, activate the `istio` input. The most basic example which is enabling collection on all namespaces except the system namespaces:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
input:
  istio:
    enabled: true
output:
    otlp:
      endpoint:
        value: http://myEndpoint:4317
```

For more details, please see [Metrics Istio Input](./istio-input.md).

## Limitations

- **Throughput**: Assuming an average metric with 20 metric data points and 10 labels, the default metric **gateway** setup has a maximum throughput of 34K metric data points/sec. If more data is sent to the gateway, it is refused. To increase the maximum throughput, manually scale out the gateway by increasing the number of replicas for the metric gateway. See [Module Configuration and Status](https://kyma-project.io/#/telemetry-manager/user/01-manager?id=module-configuration).
  The metric **agent** setup has a maximum throughput of 14K metric data points/sec per instance. If more data must be ingested, it is refused. If a metric data endpoint emits more than 50.000 metric data points per scrape loop, the metric agent refuses all the data.
- **Load Balancing With Istio**: To ensure availability, the metric gateway runs with multiple instances. If you want to increase the maximum throughput, use manual scaling and enter a higher number of instances.
  By design, the connections to the gateway are long-living connections (because OTLP is based on gRPC and HTTP/2). For optimal scaling of the gateway, the clients or applications must balance the connections across the available instances, which is automatically achieved if you use an Istio sidecar. If your application has no Istio sidecar, the data is always sent to one instance of the gateway.
- **Unavailability of Output**: For up to 5 minutes, a retry for data is attempted when the destination is unavailable. After that, data is dropped.
- **No Guaranteed Delivery**: The used buffers are volatile. If the gateway or agent instances crash, metric data can be lost.
- **Multiple MetricPipeline Support**: The maximum amount of MetricPipeline resources is 5.

## Troubleshooting and Operations

Operational remarks can be found at [Telemetry Pipeline Operations](./../pipelines/operations.md).
For pipeline troubleshooting please see [Telemetry Pipeline Troubleshooting](./../pipelines/troubleshooting.md).
