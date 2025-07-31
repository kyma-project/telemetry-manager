# Architecture

The components running in a cluster as part of the telemetry module can be seen in the following diagram and are explained in more detail in the following:

![Components](./assets/telemetry-arch.drawio.svg)

## Telemetry Manager

The Telemetry module ships Telemetry Manager as its core component. Telemetry Manager is a Kubernetes [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) that implements the Kubernetes controller pattern and manages the whole lifecycle of all other components covered in the Telemetry module. Telemetry Manager watches for the user-created Kubernetes resources: LogPipeline, TracePipeline, and MetricPipeline. In these resources, you specify what data of a signal type to collect and where to ship it.
If Telemetry Manager detects a configuration, it deploys the related gateway and agent components accordingly and keeps them in sync with the requested pipeline definition.
Summarized, the Telemetry Manager has the following tasks:

1. Watch the module configuration for changes and sync the module status to it.
2. Watch for the user-created Kubernetes resources LogPipeline, TracePipeline, and MetricPipeline. In these resources, you specify what data of a signal type to collect and where to ship it.
3. Manage the lifecycle of the self monitor and the user-configured agents and gateways.
   For example, only if you defined a LogPipeline resource, the Fluent Bit DaemonSet is deployed as log agent.

![Manager](assets/manager-resources.drawio.svg)

## Self Monitor

The Telemetry module contains a self monitor, based on [Prometheus](https://prometheus.io/), to collect and evaluate metrics from the managed gateways and agents. Telemetry Manager retrieves the current pipeline health from the self monitor and adjusts the status of the pipeline resources and the module status.
Additionally, you can monitor the health of your pipelines in an integrated backend like [SAP Cloud Logging](./integration/sap-cloud-logging/README.md#use-sap-cloud-logging-alerts): To set up alerts and reports in the backend, use the [pipeline health metrics](./04-metrics.md#5-monitor-pipeline-health) emitted by your MetricPipeline.

![Self-Monitor](assets/manager-arch.drawio.svg)

## Module Configuration and Status

For configuration options and the overall status of the module, see the specification of the related [Telemetry resource](./resources/01-telemetry.md).

## Gateways

The log, trace, and metrics features provide gateways based on an [OTel Collector](https://opentelemetry.io/docs/collector/) [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/). The gateways act as central endpoints in the cluster to which your applications push data in the [OTLP](https://opentelemetry.io/docs/reference/specification/protocol/) format.

### Log Agent

In addition to the log gateway, the additional `application` input in a LogPipeline will use the dedicated log agent based on a [DaemonSet](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/). The agent collects logs of any container printing logs to `stdout/stderr`. For more information, see [Logs](logs.md).

### Metric Agent

In addition to the metric gateway, the additional inputs in a MetricPipeline will use the dedicated metric agent based on a [DaemonSet](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/), which for example scrapes annotated Prometheus-based workloads. For more information, see [Metrics](04-metrics.md).
