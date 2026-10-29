<!-- The table below was generated automatically -->
<!-- Please do not edit it directly. If you want to change it, please update the template and regenerate the file with make target  update-metrics-docs -->
# Metrics Emitted by Kyma Telemetry Manager

| Metric                                                          | Description                                                                                   |
|-----------------------------------------------------------------|:----------------------------------------------------------------------------------------------|
| **telemetry_feature_flags_info** | Enabled feature flags in the Telemetry Manager |
| **telemetry_pipelines_metric_pipeline_info** | Feature and endpoint information of MetricPipelines |
| **telemetry_pipelines_log_pipeline_info** | Feature and endpoint information of LogPipelines |
| **telemetry_pipelines_trace_pipeline_info** | Feature and endpoint information of TracePipelines |
| **telemetry_pipelines_log_pipeline_custom_output_info** | Custom Fluent Bit output plugin in use per LogPipeline |
| **telemetry_pipelines_log_pipeline_custom_filter_info** | Custom Fluent Bit filter plugins in use per LogPipeline |
| **telemetry_self_monitor_prober_in_flight_requests** | The current number of in-flight requests initiated by the self-monitoring prober. |
| **telemetry_self_monitor_prober_requests_total** | Total number of requests initiated by the self-monitoring prober. |
| **telemetry_self_monitor_prober_duration_seconds** | A histogram of latencies for requests initiated by the self-monitoring prober. |
| **telemetry_migrator_info** | Information about the stored versions of CRDs relevant to storage migration |
| **telemetry_secret_watch_watchers_active** | The current number of active secret watchers. |
| **telemetry_secret_watch_linked_pipelines** | The total number of pipelines linked to secret watchers. |
| **telemetry_secret_watch_events_total** | Total number of secret watch events received. |
| **telemetry_secret_watch_reconnects_total** | Total number of secret watcher reconnection attempts. |
| **telemetry_node_size_smallest_memory_bytes** | The allocatable memory in bytes of the smallest node in the cluster. |
| **telemetry_service_attributes_enrichment_strategy** | Service attributes enrichment strategy used for telemetry data (either 'otel' for OpenTelemetry enrichment or 'kyma-legacy' for legacy Kyma enrichment) |
| **telemetry_build_info** | Build information of the Telemetry Manager |