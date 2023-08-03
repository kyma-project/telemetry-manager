# Telemetry Manager

## Module lifecycle

Kyma's Telemetry module on its own ships a single component only, namely the Telemetry Manager. Telemetry Manager is a Kubernetes [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) that is described by a custom resource of type Telemetry. The Telemetry Manager implements the Kubernetes controller pattern and manages the whole lifecycle of all other components relevant to the Telemetry module.
The Telemetry Manager looks out for the user-created Kubernetes resources: LogPipeline, TracePipeline, and, in the future, MetricPipeline. In these resources, you specify what data of a signal type to collect and where to ship it.
If the Telemetry Manager detects a configuration, it rolls out the relevant components on demand.

![Manager](./assets/manager-lifecycle.drawio.svg)

## Module Configuration

At the moment, you cannot configure the Telemetry Manager. It is planned to support configuration in the specification of the related [Telemetry resource](/docs/user/resources/01-telemetry.md).

## Module Status

The Telemetry Manager syncs the overall status of the module into the [Telemetry resource](/docs/user/resources/01-telemetry.md); it can be found in the `status` section. In future, the status will be enhanced with more runtime information.
