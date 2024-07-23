# Telemetry Manager

As the core element of the Telemetry module, Telemetry Manager manages the lifecycle of other Telemetry module components by watching user-created resources.

## Module Lifecycle

The Telemetry module includes Telemetry Manager, a Kubernetes [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) that's described by a custom resource of type Telemetry. Telemetry Manager has the following tasks:

1. Watch for the user-created Kubernetes resources LogPipeline, TracePipeline, and MetricPipeline. In these resources, you specify what data of a signal type to collect and where to ship it.
2. Furthermore, it watches the module configuration for changes and syncs the module status to it.
3. the Telemetry Manager takes care of the full lifecycle of the agents and gateways, related to the configured resource types. Only if, for example, a LogPipeline is defined, the Fluent Bit DaemonSet is deployed. For traces and metrics, a Kubernetes Service for pushing data to the gateways gets managed additionally. Also, the self-monitor gets managed responsible for observing the managed components and determining the module status. 

![Manager](assets/manager-resources.drawio.svg)

### Self Monitor

The Telemetry module contains a self monitor, based on [Prometheus](https://prometheus.io/), to collect and evaluate metrics from the managed gateways and agents. Telemetry Manager retrieves the current pipeline health from the self monitor and adjusts the status of the pipeline resources and the module status.

![Self-Monitor](assets/manager-arch.drawio.svg)

## Module Configuration and Status

For configuration options and the overall status of the module, see the specification of the related [Telemetry resource](./resources/01-telemetry.md).
