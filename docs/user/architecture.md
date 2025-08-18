# Architecture

The Telemetry API provides a hardened setup of an OTel Collector and also abstracts the underlying OTel Collector concept. Such abstraction has the following benefits:

- Compatibility: An abstraction layer supports compatibility when underlying features change.
- Migratability: Smooth migration experiences when switching underlying technologies or architectures.
- Native Kubernetes support: API provided by Kyma Telemetry supports an easy integration with Secrets, for example, served by the [SAP BTP Service Operator](https://github.com/SAP/sap-btp-service-operator#readme). Telemetry Manager takes care of the full lifecycle.
- Focus: The user doesn't need to understand the underlying concepts.

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
Additionally, you can monitor the health of your pipelines in an integrated backend like [SAP Cloud Logging](./integration/sap-cloud-logging/README.md#use-sap-cloud-logging-alerts): To set up alerts and reports in the backend, use the [pipeline health metrics](./metrics/health-input.md) emitted by your MetricPipeline.

![Self-Monitor](assets/manager-arch.drawio.svg)

## Gateways and Agents

The log, trace, and metrics features provide gateways based on an [OTel Collector](https://opentelemetry.io/docs/collector/) [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/). The gateways act as central endpoints in the cluster to which your applications push data in the OTLP format. From here, the data is enriched and filtered, and then dispatched configured in your pipeline resources.

- Log Gateway and Agent

  In addition to the log gateway, you can also use the log agent based on a [DaemonSet](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/), which collects logs of any container printing logs to stdout/stderr. For details, see [Logs](./logs/README.md).

  As an alternative to the OTLP-based log feature, you can choose using a log agent based on a Fluent Bit installation running as a DaemonSet. It reads all containers’ logs in the runtime and ships them according to your LogPipeline configuration. For details, see Application Logs (Fluent Bit).

- Trace Gateway

  The trace gateway provides an [OTLP](https://opentelemetry.io/docs/specs/otel/protocol/)-based endpoint to which applications can push the trace signals. Kyma modules like Istio or Serverless contribute traces transparently. For more information, see [Traces](./traces/README.md).

- Metric Gateway and Agent

  In addition to the metric gateway, you can also use the metric agent based on a [DaemonSet](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/), which, for example, scrapes annotated Prometheus-based workloads. For details, see [Metrics](./metrics/README.md).
