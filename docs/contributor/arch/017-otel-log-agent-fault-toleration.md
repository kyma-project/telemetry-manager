# 17. OTel Log Agent Fault Toleration 

Date: 2025-02-04

## Status

Proposed

## Context

### Classifying Log Loss Scenarios in an OTel Log Agent with Filelog Receiver
When setting up an OpenTelemetry (OTel) Log Agent using the Filelog Receiver, it's important to identify situations where logs might be lost and how to mitigate them.

#### Scenarios Where Data Loss Must Be Prevented:
* Temporary OTLP backend issues (e.g., spikes in retriable errors, backpressure, temporary network failures).
* Collector Pod restarts during normal operations (e.g., upgrades, rescheduling to another node).
#### Scenarios Where Preventing Data Loss Is Nice-to-Have:
* Collector Pod crashes that occur unexpectedly.
#### Scenarios Where Data Loss Is Unavoidable:
* Permanent OTLP backend failures.
* Node-level failures.
* Logs not yet tailed before Collector Pod eviction.
* Logs not yet tailed before being rotated.

### Mechanisms for Enhanced Resiliency  

#### Batch Processor

The Batch Processor accepts logs and places them into batches. Batching helps better compress the data and reduce the number of outgoing connections required to transmit the data. However, there are some problems:
* The Batch Processor asynchronously handles the incoming requests and does not propagate errors to the Filelog Receiver
* The Batch Processor doesn’t preserve its state in permanent storage, once the collector exits unexpectedly, the accumulated requests are lost. 

![Batch Processor Flow](../assets/log-agent-batch-processor-flow.svg "Batch Processor Flow")

#### Filelog Receiver Batching

The Filelog Receiver does not forward log lines to the next consumer one by one. Instead, it batches them by resource. The batch size and send interval are fixed and cannot be configured—logs are sent in batches of 100 lines or every 100 milliseconds, whichever comes first.
More info about internal details can be found [here](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/31074#issuecomment-2360284799).

This hidden feature eliminates the need for the Batch Processor, enabling a fully synchronous pipeline.

![No Batch Processor Flow](../assets/log-agent-no-batch-processor-flow.svg "No Batch Processor Flow")

#### Exporter Batcher

Exporter Batcher is a new (not yet delivered) feature that is meant to solve the limitations of the existing batch processor. It doesn’t introduce any asynchronous behavior itself but relies on the queue sender in front of it if needed. The most important is that by using a persistent queue, no data will be lost during the shutdown. It's important to note that the Exporter Batcher will not be a standalone component but rather part of the exporterhelper package, requiring integration into each exporter. However, both the Exporter Batcher and the new exporter design are still works in progress, with no clear ETA: [GitHub Issue #8122](https://github.com/open-telemetry/opentelemetry-collector/issues/8122). 

![Exporter Batcher Flow](../assets/log-agent-exporter-batcher-flow.svg "Exporter Batcher Flow")

#### Exporter Persistent Sending Queue

When persistent queue is enabled, the batches are being buffered using the provided storage extension - filestorage is a popular and safe choice. If the collector instance is killed while having some items in the persistent queue, on restart the items will be picked and the exporting is continued.

There are two types of file storage that can back a persistent queue: the node filesystem or a persistent volume (PV).

Using the node filesystem carries some risks. Misconfigurations can lead to disk overflow, potentially crashing the node or even the entire cluster. Additionally, heavy disk I/O can degrade cluster performance—something we previously observed with Fluent Bit. Another limitation is that queue size can currently only be restricted by batch count, not by volume (MB), requiring some estimations. However, the upstream project is actively working on adding size-based limits.

PV-based storage avoids these issues but cannot be used with a DaemonSet.

Overall, we have had positive experiences using node filesystem-based buffering in Fluent Bit, making it a viable solution.

#### Filelog Receiver Offset Tracking

The Filelog Receiver can persist state information on storage (typically the node’s filesystem), allowing it to recover after crashes. It maintains the following data to ensure continuity:  

- **Number of tracked files** (*knownFiles*).  
- **For each tracked file:**  
  - **File fingerprint** (*Fingerprint.first_bytes*) – a unique identifier for the file.  
  - **Byte offset** (*Offset*) – the position from which the receiver resumes reading.  
  - **File attributes** (*FileAttributes*) – metadata such as the file name.  

## Decision

Proposed solution for the OTel Log Agent setup:

### Proposed Solution for the OTel Log Agent Setup:  

**Filelog Receiver** offset tracking  
**No Batch Processor**, relying on Filelog Receiver for now  
**Exporter Batcher** (once stable)  
**Persistent Queue**, backed by the node filesystem  
