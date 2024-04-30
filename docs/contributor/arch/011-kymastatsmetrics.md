# 11. Kyma Module Status Metrics

Date: 2024-04-19

## Status

Proposed

## Context

The epic [Advanced pipeline status based on data flow](https://github.com/kyma-project/telemetry-manager/issues/425) describes efforts to make problems of the Telemetry module more transparent to users by utilizing CRD status conditions.
To ease day-two operations, the status of the Telemetry and other Kyma modules should be available as metrics. Then, users can integrate these metrics into their monitoring system; for example, by setting up alerts for a module status that differs from the expected value.

The [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics) exporter provides a way to [export metrics for custom resources](https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/extend/customresourcestate-metrics.md).
This document describes a way to integrate similar functionality to the Telemetry module's OpenTelemetry Collectors. The solution should avoid the maintenance overhead of an additional third-party-image and allow a dynamic configuration, based on the active Kyma modules.

We investigated the following existing solutions:

- The [Kubernetes Cluster Receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sclusterreceiver) is an OpenTelemetry Collector receiver with similar scope as kube-state-metrics . However, this receiver does not support monitoring custom resources.
- Another available source to monitor changes of arbitrary Kubernetes objects is the [Kubernetes Objects Receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sobjectsreceiver). This receiver produces only logs and no metrics.

- The OpenTelemetry Collector provides interfaces for enhancements by implementing custom receiver plugins. The OpenTelemetry project provides [documentation](https://opentelemetry.io/docs/collector/building/receiver/) on implementing a custom receiver and adding it to a custom distribution of the Collector.

## Decision

Due to the restrictions of available telemetry resources for Kubernetes resources, building a custom receiver is the most suitable option.

### Extended MetricPipeline API

To activate module status metrics as a new input, the MetricPipeline CRD needs a _module list_. If the module list is empty, metrics of all active modules are collected. As in the following example, users can select the Kyma modules of interest:

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

Enabling the Kyma input will enable a custom metrics receiver, called `kymastats`, in the Telemetry metric gateway that produces metrics for the module state and status conditions.

The receiver configuration will follow the shown example:

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

We assume that the status subresource of a module CRD contains a `conditions` list that uses the type [meta/v1/Condition](https://pkg.go.dev/k8s.io/apimachinery@v0.30.0/pkg/apis/meta/v1#Condition), and an overarching state attribute. We assume positive polarity for all conditions.

As an example for this structure, see the `status` subresource of the Telemetry module:

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

| Metric Name                  | Attributes                 | Description                                                                                                           |
|------------------------------|----------------------------|-----------------------------------------------------------------------------------------------------------------------|
| kyma.module.status.state     | state, name                | Reflects .status.state field of the module CRD. Value is 1 if the state is `Ready`, else 0.                           |
| kyma.module.status.condition | reason, status, name, type | Exports condition status of all conditions under .status.conditions. Value is 1 if the condition status is 1, else 0. |

Collecting the module specific metrics should continue working in the case of a Node or Pod failure (high availability) without emitting metrics multiple times. To ensure this behavior, Kubernetes API server [leases](https://kubernetes.io/docs/concepts/architecture/leases/) can be used while only the lease holder should emit metrics. We will investigate for a generic solution in the OpenTelemetry Collector.

The status of the Kyma CR should not be exported as a metric by the described approach. The receiver should also work with individually installed modules that are not managed by the lifecycle manager. The synchronization of the Kyma CR to module CRs is considered to be out of scope for this end-user facing metrics.
