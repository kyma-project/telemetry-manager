# 11. Kyma Module Status Metrics

Date: 2024-04-19

## Status

Proposed

## Context

The [Advanced pipeline status based on data flow](https://github.com/kyma-project/telemetry-manager/issues/425) epic describes efforts to make problems of the Telemetry module more transparent to users by utilizing CRD status conditions.
To ease day-two operations, the status of the Telemetry and other Kyma modules should by available as metrics to enable users to integrate them into their monitoring system. For instance, by setting up alerts for a module status that differs from the expected value.

The [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics) exporter provides a way to [export metrics for custom resources](https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/extend/customresourcestate-metrics.md).
This record describes a way to integrate similar functionality to the Telemetry module's OpenTelemetry Collectors. The solution should avoid the maintenance overhead of an additional third-party-image and allow a dynamic configuration, based on the active Kyma modules.

An OpenTelemetry Collector receiver with similar scope as kube-state-metrics is the [Kubernetes Cluster Receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sclusterreceiver). However, this receiver does not support monitoring custom resources.
Another available source to monitor changes of arbitrary Kubernetes objects is the [Kubernetes Objects Receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sobjectsreceiver). This receiver produces only logs and no metrics.

The OpenTelemetry Collector provides interfaces for enhancements by implementing custom receiver plugins. The OpenTelemetry project provides [documentation](https://opentelemetry.io/docs/collector/building/receiver/) on implementing a custom receiver and adding it to a custom distribution of the Collector.

## Decision

Due to the restrictions of available telemetry resources for Kubernetes resources, building a custom receiver is the most suitable option.

### Extended MetricPipeline API

Module status metrics can be activated as a new input in the MetricPipeline CRD. Users can select the Kyma modules of interest by giving a module list. An empty list will include metrics of all active modules.

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: sample
spec:
  input:
    kyma:
      enabled: true
      modules:
        - telemetry
    runtime:
      enabled: true
    prometheus:
      enabled: true
    istio:
      enabled: true
  output:
    otlp:
      endpoint:
        value: http://example.com:4317
```

Enabling the Kyma input will enable a custom metrics receiver, called `kymastats`, in the telemetry-metric-gateway that produces metrics for the module state and status conditions.
The receiver configuration will follow the shown example.

```yaml
receivers:
  kymastats:
    k8s_cluster:
        auth_type: serviceAccount
    collection_interval: 30s
    api_groups:
    - operator.kyma-project.io
```

The receiver needs the following properties:

- **auth_type**: The way to authenticate with the Kubernetes API. Possible values are `serviceAccount` (default) or `kubeConfig`.
- **collection_interval**: The interval that is used to emit metrics.
- **api_groups**: List of API groups to scrape. Most of the Kyma modules should use the `operator.kyma-project.io` API group. The list can be extended to support monitoring custom modules. Every CRD in the listed groups is assumed to represent a module.

### Custom Metrics Receiver for OpenTelemetry Collector

We assume the status subresource of a module CRD to contain a `conditions` list that uses the [meta/v1/Condition](https://pkg.go.dev/k8s.io/apimachinery@v0.30.0/pkg/apis/meta/v1#Condition) type and an overarching state attribute. We assume positive polarity for all conditions.

An example for this structure is the status subresource of the Telemetry module:

```yaml
status:
  conditions:
  - lastTransitionTime: "2024-04-18T13:43:03Z"
    message: All log components are running
    observedGeneration: 2
    reason: LogComponentsRunning
    status: "True"
    type: LogComponentsHealthy
  - lastTransitionTime: "2024-04-18T13:41:55Z"
    message: All metric components are running
    observedGeneration: 2
    reason: MetricComponentsRunning
    status: "True"
    type: MetricComponentsHealthy
  - lastTransitionTime: "2024-04-15T12:36:47Z"
    message: All trace components are running
    observedGeneration: 2
    reason: TraceComponentsRunning
    status: "True"
    type: TraceComponentsHealthy
  endpoints:
    metrics:
      grpc: http://telemetry-otlp-metrics.kyma-system:4317
      http: http://telemetry-otlp-metrics.kyma-system:4318
    traces:
      grpc: http://telemetry-otlp-traces.kyma-system:4317
      http: http://telemetry-otlp-traces.kyma-system:4318
  state: Ready
```

Additional attributes, like the `endpoints` of the Telemetry status, are ignored.

Status and conditions should result in the following metrics:

| Metric Name                                   | Attributes     | Description                                                                                                           |
|-----------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------|
| kyma_module_\<name>_status_state              | state          | Reflects .status.state field of the module CRD. Value is 1 if the state is `Ready`, else 0.                           |
| kyma_module_\<name>_status_condition\_\<type> | reason, status | Exports condition status of all conditions under .status.conditions. Value is 1 if the condition status is 1, else 0. |
