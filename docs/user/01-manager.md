# Telemetry Manager

## Module Lifecycle

Kyma's Telemetry module ships Telemetry Manager as its core component. Telemetry Manager is a Kubernetes [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) that is described by a custom resource of type Telemetry. Telemetry Manager implements the Kubernetes controller pattern and manages the whole lifecycle of all other components covered in the Telemetry module.
Telemetry Manager watches for the user-created Kubernetes resources: LogPipeline, TracePipeline, and MetricPipeline. In these resources, you specify what data of a signal type to collect and where to ship it.
If Telemetry Manager detects a configuration, it rolls out the relevant components on demand.

![Manager](assets/manager-lifecycle.drawio.svg)

## Module Configuration

The [Telemetry resource](resources/01-telemetry.md) allows to configure the number of replicas for the `telemetry-trace-gateway` and `telemetry-metric-gateway` deployments. The default value is 2.

## Module Status

Telemetry Manager syncs the overall status of the module into the [Telemetry resource](resources/01-telemetry.md); it can be found in the `status` section. In future, the status will be enhanced with more runtime information.
