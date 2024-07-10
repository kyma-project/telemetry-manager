# Telemetry Manager

As the core element of the Telemetry module, Telemetry Manager manages the lifecycle of other Telemetry module components by watching user-created resources.

## Module Lifecycle

The Telemetry module includes Telemetry Manager, a Kubernetes [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) that's described by a custom resource of type Telemetry. Telemetry Manager has the following tasks:

1. Watch for the user-created Kubernetes resources LogPipeline, TracePipeline, and MetricPipeline. In these resources, you specify what data of a signal type to collect and where to ship it.
2. If it finds such a custom resource: Roll out the relevant components on demand and keep it in sync with the pipeline.

![Manager](assets/manager-lifecycle.drawio.svg)

### Self Monitor

The Telemetry module contains a self monitor, based on [Prometheus](https://prometheus.io/), to collect and evaluate metrics from the managed gateways and agents. Telemetry Manager retrieves the current pipeline health from the self monitor and adjusts the status of the pipeline resources and the module status.

![Manager](assets/manager-arch.drawio.svg)

## Module Configuration and Status

For configuration options and the overall status of the module, see the specification of the related [Telemetry resource](./resources/01-telemetry.md).
