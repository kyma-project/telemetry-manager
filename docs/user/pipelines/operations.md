# Operations

A telemetry pipeline runs several OTel Collector instances in your cluster.

The Telemetry module ensures that the OTel Collector instances are operational and healthy at any time, for example, with buffering and retries. However, there may be situations when the instances drop logs, or cannot handle the log load.

To detect and fix such situations, check the [LogPipeline](./../resources/02-logpipeline.md#logpipeline-status)|[MetricPipeline](./../resources/05-metricpipeline.md#metricpipeline-status)|[TracePipeline](./../resources/04-tracepipeline.md#tracepipeline-status) status and check out [Troubleshooting](./troubleshooting.md). If you have set up [pipeline health monitoring](./../metrics/health-input.md), check the alerts and reports in an integrated backend like [SAP Cloud Logging](./../integration/sap-cloud-logging/README.md#use-sap-cloud-logging-alerts).

> [! WARNING]
> It's not recommended to access the metrics endpoint of the used OTel Collector instances directly, because the exposed metrics are no official API of the Telemetry module. Breaking changes can happen if the underlying OTel Collector version introduces such.
> Instead, use the eachs pipeline status.
