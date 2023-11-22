# 1. Fluent Bit without File-System Buffer

Date: 2023-11-23

## Status

Proposed

## Context

The Fluent Bit configuration that we currently generate with the telemetry-manager from a LogPipeline uses a `tail`
input plugin to read container logs, splits the log stream into multiple pipelines using a `rewrite_tag` filter, applies
additional pipeline specific filters, and sends the log stream to the configured output. This setup decouples the input
and outputs so that a blocked output does not affect all other outputs. Consequently, reading logs by the tail plugin is
never paused. The `rewrite_tag` filter uses a persistent file-system buffer to prevent log loss. The persistent buffer
is limited to 1 GB to ensure that Kubernetes nodes do not run out of disk space.

The persistent buffer can ensure log loss only for a short period of time. Depending on the amount of generated logs,
this is usually in the range of minutes to a few hours and might be too short to restore an outage of a log backend or
detect and solve a faulty configuration.

The removal of the in-cluster Loki backend requires a reevaluation of the pipeline isolation requirements in favor of
pausing the input plugin. We consider clusters with a single LogPipeline or multiple LogPipelines that ingest logs to
the same backend as the typical setup. For instance, ingesting application logs and Istio access logs into two different
indexes of a [SAP Cloud Logging](../../user/integration/sap-cloud-logging) instance. With this assumption, the benefit
of pausing the `tail` input plugin exceeds the pipeline isolation requirements. Kubernetes' [logging
architecture](https://kubernetes.io/docs/concepts/cluster-administration/logging/) helps by storing logs persistently on
the node file-system and rotating automatically after reaching a certain file size.

## Decision

Fluent Bit can be configured in different ways to read container logs and ingest them into multiple backends:

1. **Single log stream:** Logs are read by a `tail` input plugin, one or more filters are applied, and written to the
   backend by multiple output plugins. This setup does not allow to control the input based in the condition of
   individual outputs. Filters cannot be applied per individual pipeline. The complexity of this setup is low.
2. **Split log stream**: Logs are read by a `tail` input plugin, global filters like the Kubernetes filter are applied
   before the stream is split into a pipeline specific stream. Each stream can have additional filters before logs are
   written to an output. This setup is currently used by the telemetry-manager. Pipelines can be isolated using the
   persistent buffer. However, pausing the input in the situation of a backend outage is not possible. The complexity of
   this setup is high.
3. **Dedicated log streams**: Each pipeline has its own `tail` input plugin, a list of filters, and output plugin. This
   setup isolates the log processing between different pipelines and allows to pause the streams individually in the
   case of a problem. This setup has medium complexity.
4. **Dedicated Fluent Bit instances**: Each pipeline gets its own Fluent Bit DaemonSet. This setup isolates also the CPU
   and memory resources per pipeline with the cost of a larger overhead. This setup has medium complexity.

Option 1 does not fulfill our requirement to apply individual filters per pipeline. Option 4 will cause for our typical
setup of two pipelines (application logs and access logs) an unacceptable resource overhead. Option 2 and 3 allow log
filter settings per pipeline.

We consider option 3 to be the best Fluent Bit configuration for our requirements because of its lower complexity and
lower risk for log loss. The throughput of option 3 has shown to be better than option 2 without changing Fluent Bit's
CPU and memory limits.

## Consequences

Switching from option 2 to option 3 relies on the container runtime for persistent log storage. This changes the
conditions under which logs might be lost. Option 2 with its persistent buffer of 1 GB per pipeline deletes the
oldest logs when the limit is reached. For option 3, the conditions under which logs might be lost are specific for the
individual pod since log rotation happens when then log file per pod reach a certain size. Logs are deleted in the case
of pod evictions or multiple restarts.
