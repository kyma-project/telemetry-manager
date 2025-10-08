# Collecting Metrics

The Telemetry module collects metrics from your workloads and Kubernetes resources. Use these metrics to monitor health, performance, and behavior. To start collecting metrics, create a `MetricPipeline` resource. You can collect Prometheus, Istio, and runtime metrics.

## Overview

The `MetricPipeline` is a Kubernetes Custom Resource (CR). It configures metric collection for your cluster. When you create a `MetricPipeline`, it automatically provisions these components:

- A metric gateway: Provides a central OTLP endpoint. Applications push metrics to this endpoint.
- A metric agent: Runs on each cluster node. It scrapes metrics from applications and Kubernetes resources.

The pipeline enriches all collected metrics with Kubernetes metadata. It also transforms non-OTLP formats (like Prometheus) into the OTLP standard before sending them to your chosen backend.

Metrics collection is optional. If you do not create a `MetricPipeline`, the system does not deploy metric collection components.

## Prerequisites

Before you collect metrics data from a component, it must expose (instrument) the metrics. Typically, a component instruments specific metrics for its language runtime (for example, Node.js) and custom metrics for its business logic. Metrics exposure can be in different formats, such as the pull-based Prometheus format or the push-based OTLP format.

If you use Prometheus-based metrics, instrument your application with a library like the Prometheus client library. Expose a port in your workload to serve as a Prometheus metrics endpoint.

If you scrape the metric endpoint with Istio, define the app protocol in your Service port definition. See [Collect Prometheus Metrics](prometheus-input.md).

For instrumentation, use an SDK, such as the Prometheus client libraries or the OpenTelemetry SDKs. Both libraries provide extensions to activate language-specific auto-instrumentation (for example, for Node.js) and an API to implement custom instrumentation.

## Minimal MetricPipeline

For a minimal setup, create a `MetricPipeline` that specifies your backend destination. See [Integrate With Your OTLP Backend](../pipelines/otlp-input.md).

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

By default, this minimal pipeline collects these types of metrics:

- OTLP Metrics: Activates cluster-internal endpoints to receive metrics in the OTLP format. Applications push metrics directly to these URLs:
  - gRPC: `http://telemetry-otlp-metrics.kyma-system:4317`
  - HTTP: `http://telemetry-otlp-metrics.kyma-system:4318`
- Health Metrics: Collects health and performance metrics about the Telemetry module's components. This input is always active; you cannot disable it. See [Monitor Pipeline Health](ADD LINK).

To collect metrics from Kyma modules like Istio, Eventing, or Serverless, enable additional inputs.

## Configure Metrics Collection

You can adjust the MetricPipeline using runtime configuration with the available parameters (see [MetricPipeline: Custom Resource Parameters](https://kyma-project.io/#/telemetry-manager/user/resources/05-metricpipeline?id=custom-resource-parameters).

- Scrape `prometheus` metrics from applications that expose a Prometheus-compatible endpoint. See [Collect Prometheus Metrics](prometheus-input.md).
- Collect `istio` service mesh metrics from Istio proxies and control plane components. See [Collect Istio Metrics](istio-input.md).
- Collect `runtime` resource usage and status metrics from Kubernetes components like Pods, Nodes, and Deployments. See [Collect Runtime Metrics](runtime-input.md).
- Use diagnostic metrics to debug your `prometheus` and `istio` configuration. See [Collect Diagnostic Metrics](prometheus-input.md#diagnostic-metrics).
- Choose specific namespaces to include or drop metrics. See [Filter Metrics](ADD LINK).
- Avoid redundancy by dropping push-based OTLP metrics sent directly to the metric gateway. See [Route Specific Inputs to Different Backends](../pipelines/otlp-input.md#route-specific-inputs-to-different-backends).

## Limitations

- **Throughput**: Assuming an average metric with 20 metric data points and 10 labels, the default metric **gateway** setup has a maximum throughput of 34K metric data points/sec. If more data is sent to the gateway, it is refused. To increase the maximum throughput, manually scale out the gateway by increasing the number of replicas for the metric gateway. See [Module Configuration and Status](https://kyma-project.io/#/telemetry-manager/user/01-manager?id=module-configuration).
  The metric **agent** setup has a maximum throughput of 14K metric data points/sec per instance. If more data must be ingested, it is refused. If a metric data endpoint emits more than 50.000 metric data points per scrape loop, the metric agent refuses all the data.
- **Load Balancing With Istio**: To ensure availability, the metric gateway runs with multiple instances. If you want to increase the maximum throughput, use manual scaling and enter a higher number of instances.
  By design, the connections to the gateway are long-living connections (because OTLP is based on gRPC and HTTP/2). For optimal scaling of the gateway, the clients or applications must balance the connections across the available instances, which is automatically achieved if you use an Istio sidecar. If your application has no Istio sidecar, the data is always sent to one instance of the gateway.
- **Unavailability of Output**: For up to 5 minutes, a retry for data is attempted when the destination is unavailable. After that, data is dropped.
- **No Guaranteed Delivery**: The used buffers are volatile. If the gateway or agent instances crash, metric data can be lost.
- **Multiple MetricPipeline Support**: The maximum amount of MetricPipeline resources is 5.
