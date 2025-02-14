# 17. Fault Tolerant OTel Logging Setup

Date: 2025-02-14

## Status

Proposed

## Context

### Classifying Log Loss Scenarios
When setting up an OpenTelemetry (OTel) Log Agent using the Filelog Receiver, it's important to identify situations where logs might be lost and how to mitigate them.

#### Scenarios Where Data Loss Must Be Prevented:
- Temporary OTLP backend issues (for example, spikes in retriable errors, backpressure, temporary network failures).
- Collector Pod restarts during normal operations (for example, upgrades, rescheduling to another node).

#### Scenarios Where Preventing Data Loss Is Nice-to-Have:
- Collector Pod crashes that occur unexpectedly.

#### Scenarios Where Data Loss Is Unavoidable:
- Permanent OTLP backend failures.
- Node-level failures.
- Logs not yet tailed before Collector Pod eviction.
- Logs not yet tailed before being rotated.

### Mechanisms for Enhanced Resiliency  

#### Batching

##### Batch Processor
The Batch Processor accepts logs and places them into batches. Batching helps better compress the data and reduce the number of outgoing connections required to transmit the data. However, there are some problems:
- The Batch Processor asynchronously handles the incoming requests and does not propagate errors to the Filelog Receiver
- The Batch Processor doesn’t preserve its state in permanent storage. Once the collector exits unexpectedly, the accumulated requests are lost. 

![Batch Processor Flow](../assets/log-agent-batch-processor-flow.svg "Batch Processor Flow")

##### Filelog Receiver Batching
The Filelog Receiver does not forward log lines to the next consumer one by one. Instead, it batches them by resource. The batch size and send interval are fixed and cannot be configured - logs are sent in batches of 100 lines or every 100 milliseconds, whichever comes first.
For more information about internal details, see  [this comment](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/31074#issuecomment-2360284799).

This hidden feature eliminates the need for the Batch Processor, enabling a fully synchronous pipeline.

![No Batch Processor Flow](../assets/log-agent-no-batch-processor-flow.svg "No Batch Processor Flow")

##### Exporter Batcher
> [!NOTE]
> Both the Exporter Batcher and the new exporter design are still works in progress, with no clear ETA: [GitHub Issue #8122](https://github.com/open-telemetry/opentelemetry-collector/issues/8122). However, our tests show that it already works as expected.

Exporter Batcher is a new feature that solves the limitations of the existing batch processor. It doesn’t introduce any asynchronous behavior itself but relies on the queue sender in front of it if needed. The most important feature is that by using a persistent queue, no data is lost during the shutdown. It's important to note that the Exporter Batcher is not a standalone component but part of the `exporterhelper` package, requiring integration into each exporter.

![Exporter Batcher Flow](../assets/log-agent-exporter-batcher-flow.svg "Exporter Batcher Flow")

##### Conclusion  

Overall, `Exporter Batcher` is a future-proof solution that works reliably despite being experimental. Enabling it for all three gateway types and possibly the metric agent makes sense. However, it is not needed for the log agent, as the Filelog Receiver already handles pre-batching.  

#### Queueing  

The OTLP exporter can buffer telemetry items before sending them to the backend, using either a persistent or in-memory queue.  

##### Memory Queue  

With an in-memory queue enabled, batches do not persist across restarts. The collector implements graceful termination, meaning that the queue is drained on shutdown. However, only a single retry is attempted for each item, regardless of the `retry_sender` configuration. If that retry fails, the data is dropped.  

##### Persistent Queue  

When a persistent queue is enabled, batches are buffered using the configured storage extension —`filestorage` being a popular and reliable choice. If the collector instance is killed while holding items in the persistent queue, those items are retained and exported upon restart.  

A persistent queue can be backed by two types of file storage: the node’s filesystem or a persistent volume (PV).  

###### Node Filesystem-Based Storage  

We have had positive experiences with node filesystem-based buffering in Fluent Bit, but there are several limitations:  
- The node’s filesystem has limited storage capacity.  
- Misconfigurations can lead to disk overflows, potentially crashing the node or even the entire cluster.  
- Heavy disk I/O can degrade cluster performance—an issue we previously observed with Fluent Bit.  
- Queue size can currently only be limited by batch count, not by volume (MB), requiring rough estimations. However, the upstream project is actively working on adding size-based limits. See [opentelemetry-collector#9462](https://github.com/open-telemetry/opentelemetry-collector/issues/9462).

###### Persistent Volume (PV)-Based Storage  

Using PV-based storage mitigates these issues but introduces other constraints:  
- It cannot be used with a DaemonSet, only a StatefulSet.  
- Disk reattachment is unstable on Azure, causing simple operations like rolling upgrades to take excessively long or get stuck.  

#### Conclusion  

An in-memory queue poses a risk of data loss if the backend struggles, as data is retried only once during draining. A PV-based persistent queue is not a viable option due to operational challenges on Azure. While node filesystem-based storage can be used on Azure clusters, it is very limited and only suitable for the log agent, not the gateway.

Given these constraints, the proposal is to use an in-memory queue for the log gateway, while for the log agent, we may consider disabling it entirely.

#### Filelog Receiver Offset Tracking
The Filelog Receiver can persist state information on storage (typically the node’s filesystem), allowing it to recover after crashes. It maintains the following data to ensure continuity:  

- **Number of tracked files** (*knownFiles*).  
- **For each tracked file:**  
  - **File fingerprint** (*Fingerprint.first_bytes*) – a unique identifier for the file.  
  - **Byte offset** (*Offset*) – the position from which the receiver resumes reading.  
  - **File attributes** (*FileAttributes*) – metadata such as the file name.  

### Agent-to-Backend vs Agent-to-Gateway-to-Backend
Let's compare the following architectures Agent-to-Backend and Agent-to-Gateway-to-Backend. Each has its own trade-offs.

#### Agent-to-Backend
Direct communication between the agent and the backend. Gateway is optional and only handles OTel logs.

Pros:

- Reduced latency.
- Direct communication ensures minimal failure points.
- If the backend enforces per-connection rate limiting, only high-volume agents get throttled. It can be beneficial since often most workloads in a cluster generate minimal logs, while a few may produce a large volume.
- A multi-pipeline setup in the agent can be implemented more naturally, with a single [OTel pipeline](https://opentelemetry.io/docs/collector/architecture/#pipelines) per LogPipeline.

Cons:  

- Every gateway enrichment feature must be duplicated in the agent and tested separately.  
- Credential rotations require a full DaemonSet rollout and restart.
- Potentially higher costs, as each agent must implement the full telemetry pipeline.  

#### Agent-to-Gateway-To-Backend
Pros:

- Separation of concerns (agents collect node-affine data, gateway is responsible for filtering, enrichment, credential management, exporting to backend).
- Lighter agents, as all enrichment is handled in the gateway (for both file-based and OTel logs).

Cons:

- The gateway can become a bottleneck for all agents if a few have a high export rate.
- The gateway lacks auto-scaling; manual scaling may not be sufficient.
- Introduces an additional network hop, increasing latency.

## Decision
Proposed solution for the OTel log agent setup:
- Filelog Receiver offset tracking.
- No Batch Processor.
- No sending queue/in-memory sending queue/node filesystem sending queue.
- Agent-to-Backend communication.

Proposed solution for the OTel log gateway setup:
- No Batch Processor.
- Exporter Batcher is necessary because there is no pre-batching.
- In-memory sending queue.
