# Telemetry Pipeline Operations

The Telemetry module ensures that the OTel Collector instances of your pipelines are operational and healthy at any time, for example, with buffering and retries. However, there may be situations when the instances drop data or cannot handle the load, and you must take action.

## Health Status

The Telemetry module monitors the metrics from the managed gateways and agents, and accordingly sets the health status of the module and your pipeline resources. For details, see [Telemetry Self Monitor](./../architecture.md#self-monitor).

To detect situations when the instances drop data or cannot handle the load, use the respective status conditions for your pipelines:

- [LogPipeline Status](./../resources/02-logpipeline.md#logpipeline-status)
- [TracePipeline Status](./../resources/04-tracepipeline.md#tracepipeline-status)
- [MetricPipeline Status](./../resources/05-metricpipeline.md#metricpipeline-status)

Or verify the overall status of the module, see [Telemetry Status](./../resources/01-telemetry.md#telemetry-cr-state)

## Health Monitoring

> [! WARNING]
> It's not recommended to access the metrics endpoint of the used OTel Collector instances directly, because the exposed metrics are no official API of the Telemetry module. Breaking changes can happen if the underlying OTel Collector version introduces such.
> Instead, use the eachs pipeline status.

We recommend that you set up pipeline health monitoring and alert rules. Monitor the alerts and reports in an integrated backend – for an example, see [Integrate with SAP Cloud Logging](./../integration/sap-cloud-logging/).

By default, a MetricPipeline emits metrics about the health of all pipelines managed by the Telemetry module. Based on these metrics, you can track the status of every individual pipeline and set up alerting for it, see [Metrics Health Input](./../metrics/health-input.md).

The Telemetry module ensures that the OTel Collector instances are operational and healthy at any time, for example, with buffering and retries. However, there may be situations when the instances drop logs, or cannot handle the log load.

To detect and fix such situations, check the [LogPipeline](./../resources/02-logpipeline.md#logpipeline-status)|[MetricPipeline](./../resources/05-metricpipeline.md#metricpipeline-status)|[TracePipeline](./../resources/04-tracepipeline.md#tracepipeline-status) status and check out [Troubleshooting](./troubleshooting.md). If you have set up a MetricPipeline, it will be default collect health metrics, see [Metrics Health Input](./../metrics/health-input.md). So check the alerts and reports in an integrated backend like [SAP Cloud Logging](./../integration/sap-cloud-logging/README.md#use-sap-cloud-logging-alerts).

## Diagnostic Metrics

Consider activating diagnostic metrics. By default, they are disabled.

If you use the `prometheus` or `istio` input of a MetricPipeline, for every metric source typical scrape metrics are produced.
If you want to use them for debugging and diagnostic purposes, define a MetricPipeline that has the diagnosticMetrics section for the respective input defined, see [Metrics Prometheus Input](./../metrics/prometheus-input.md) and [Metrics Istio Input](./../metrics/istio-input.md).
