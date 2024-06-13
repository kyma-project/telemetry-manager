# 10. Log Flow Health Status API

Date: 2024-03-28

## Status

Proposed

## Context: Key Events in the Fluent Bit Log Flow

[ADR 003: Integrate Prometheus With Telemetry Manager Using Alerting](003-integrate-prometheus-with-telemetry-manager-using-alerting.md) describes a concept for self-monitoring and [ADR 008 Telemetry Flow Health Status API](008-telemetry-flow-healthiness-status-api.md) defines the related pipeline conditions that are derived from the OpenTelemetry Collector metrics. This ADR focuses on events in the Fluent Bit log flow.

![Fluent Bit Data Flow](../assets/fluent-bit-data-flow.drawio.svg "Fluent Bit Data Flow")

### Log Rotation

Container logs are rotated and finally removed by the kubelet. Logs that have not been read by Fluent Bit before rotation are lost. If logs are lost because of that, there is little to no indication (metrics, etc.).

### High Buffer Usage

After reading logs from the host file-system, the tail input plugin writes them the a persistent buffer. The buffer has a limited capacity and can fill up if logs are read faster than they can be sent to the backend. If the buffer is full and the tail input plugin keeps reading, the oldest logs are dropped.

### Backend Throttling

Each logging backend has an ingestion rate limit. The backend's maximum ingestion rate is propagated to Fluent Bit's output plugins - either by blocking all output threads due to a slow response, or by returning errors, which require the output to perform retries. Utilization of the file-system buffer indicates backend throttling.

## Decision

For the pipeline health condition type, the **reason** field can show a value that is most relevant for the user. We suggest the following values, ordered from most to least critical:

   ```
   AllTelemetryDataDropped > SomeTelemetryDataDropped > NoLogsDelivered > BufferFillingUp > Healthy
   ```

Note that the `NoLogsDelivered` reason is unique to LogPipeline custom resource and does not apply to MetricPipeline and Trace Pipeline resources. This is because a log chunk can remain in the Fluent Bit buffer for a few  days while being retried, giving customers time to rectify issues before logs are dropped. In contrast, with the OTel Collector, data is retried for only a few minutes before being immediately dropped. Thus, customers have no opportunity to react, making a special reason unnecessary.

The reasons are based on the following alert rules:

| Alert Rule | Expression |
| --- | --- |
| AgentExporterSendsLogs         | `sum(rate(fluentbit_output_bytes_total{...}[5m])) > 0`           |
| AgentReceiverReadsLogs         | `sum(rate(fluentbit_input_bytes_total{...}[5m])) > 0`        |
| AgentExporterDroppedLogs       | `sum(rate(fluentbit_output_retries_failed_total{...}[5m])) > 0`    |
| AgentBufferInUse               | `telemetry_fsbuffer_usage_bytes{...}[5m] > 300000000` |
| AgentBufferFull                | `telemetry_fsbuffer_usage_bytes[5m] > 900000000`   |

Then, we map the alert rules to the reasons as follows:

| Reason | Alert Rules |
| --- | --- |
| AllTelemetryDataDropped           | **not** AgentExporterSendsLogs **and** AgentBufferFull |
| SomeTelemetryDataDropped          | AgentExporterSendsLogs **and** AgentBufferFull       |
| NoLogsDelivered                   | **not** AgentExporterSendsLogs **and** AgentReceiverReadsLogs |
| BufferFillingUp                   | AgentBufferInUse |
| Healthy                           | **not** (AgentBufferInUse **or** AgentBufferFull) **and** (**not** AgentReceiverReadsLogs **or** AgentExporterSendsLogs) |

The metrics related to file-system buffer are not mappable to a particular LogPipeline. Thus, Telemetry Manager must set the condition on all pipelines if file-system buffer usage is indicated by the metrics.
