# 2. Fluent Bit Configuration and File-System Buffer Usage

Date: 2023-11-23

## Status

Accepted

## Context

The Fluent Bit configuration that the Telemetry Manager currently generates from a LogPipeline uses a `tail` input plugin to read container logs, splits the log stream into multiple pipelines using a `rewrite_tag` filter, applies additional pipeline specific filters, and sends the log stream to the configured output. This setup decouples the input and outputs so that a blocked output does not affect any other outputs. Consequently, reading logs by the tail plugin is never paused. The `rewrite_tag` filter uses a persistent file-system buffer to prevent log loss. The persistent buffer is limited to 1 GB to ensure that Kubernetes nodes do not run out of disk space.

The persistent buffer can prevent log loss only for a short period of time. Depending on the amount of generated logs, this is usually in the range of minutes to a few hours and might be too short to restore an outage of a log backend or detect and solve a faulty configuration.

After the in-cluster Loki backend has been removed, the pipeline isolation requirements must be reevaluated in favor of pausing the input plugin. The typical setup are clusters with a single LogPipeline or multiple LogPipelines that ingest logs to the same backend; for example, ingesting application logs and Istio access logs into two different indexes of an [SAP Cloud Logging](../../user/integration/sap-cloud-logging) instance. With this typical setup, the benefit of pausing the `tail` input plugin exceeds the pipeline isolation requirements. Kubernetes' [logging architecture](https://kubernetes.io/docs/concepts/cluster-administration/logging/) helps by storing logs persistently on the node file-system and rotating them automatically after reaching a certain file size.

## Decision

Fluent Bit can be configured in different ways to read container logs and ingest them into multiple backends:

1. **Single log stream:** Logs are read by a `tail` input plugin, one or more filters are applied, and written to the backend by multiple output plugins. The input cannot be controlled based on the condition of individual outputs. Filters cannot be applied per individual pipeline. The complexity of this setup is low.
2. **Split log stream**: Logs are read by a `tail` input plugin. Before the stream is split into a pipeline-specific stream, global filters like the Kubernetes filter are applied; and before logs are written to an output, each stream can have additional filters. Pipelines can be isolated using the persistent buffer. However, it's impossible to pause the input in the situation of a backend outage. The complexity of this setup is high.
3. **Dedicated log streams**: Each pipeline has its own `tail` input plugin, a list of filters, and an output plugin. This setup isolates the log processing between different pipelines. In the case of a problem, the streams can be paused individually. This setup has medium complexity.
4. **Dedicated Fluent Bit instances**: Each pipeline gets its own Fluent Bit DaemonSet. This setup isolates also the CPU and memory resources per pipeline with the cost of a larger overhead. This setup has medium complexity.

Currently, Telemetry Manager uses option 2. 
Option 1 does not fulfill our requirement to apply individual filters per pipeline. Option 4 causes an unacceptable resource overhead for our typical setup of two pipelines (application logs and access logs). Option 2 and 3 allow log filter settings per pipeline.

We consider option 3 to be the best Fluent Bit configuration for our requirements because of its lower complexity. The throughput of option 3 has shown to be better than option 2 without changing Fluent Bit's CPU and memory limits.
The persistent file-system buffer is still considered to be useful over pausing the `tail` input because the stored amount of logs per individual Pod is significantly lower with Kubernetes' built-in log storage. The directory-size-exporter also provides a better observability of log loss.

## Consequences

Switching from option 2 to option 3 relies on the container runtime for persistent log storage. This changes the conditions under which logs might be lost: Option 2, with its persistent buffer of 1 GB per pipeline, deletes the oldest logs when the limit is reached. For option 3, the conditions under which logs might be lost are specific for the individual pod, because log rotation happens when the log file per pod reaches a certain size. Logs are deleted in the case of pod evictions or multiple restarts. Node deletion might also be a reason for log loss. However, because the persistent buffer of the existing setup uses the host file system, this risk is already existing.
