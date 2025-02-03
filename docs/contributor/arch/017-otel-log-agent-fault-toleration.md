# 17. OTel Log Agent Fault Toleration 

Date: 2025-02-04

## Status

Proposed

## Context

## Decision

### Classifying Log Loss Scenarios in an OTel Log Agent with Filelog Receiver
When setting up an OpenTelemetry (OTel) Log Agent using the Filelog Receiver, it's important to identify situations where logs might be lost and how to mitigate them.

#### Scenarios Where Data Loss Must Be Prevented:
* Temporary OTLP backend issues (e.g., spikes in retriable errors, backpressure, temporary network failures).
#### Scenarios Where Preventing Data Loss Is Beneficial:
* Collector Pod restarts during normal operations (e.g., upgrades, rescheduling to another node).
#### Scenarios Where Preventing Data Loss Is Nice-to-Have:
* Collector Pod crashes that occur unexpectedly.
#### Scenarios Where Data Loss Is Unavoidable:
* Permanent OTLP backend failures.
* Node-level failures.
* Logs not yet tailed before Collector Pod eviction.
* Logs not yet tailed before being rotated.

### Mechanisms for Enhanced Resiliency  

#### Filelog Receiver Offset Tracking

The Filelog Receiver can persist state information on storage (typically the node’s filesystem), allowing it to recover after crashes. It maintains the following data to ensure continuity:  

- **Number of tracked files** (*knownFiles*).  
- **For each tracked file:**  
  - **File fingerprint** (*Fingerprint.first_bytes*) – a unique identifier for the file.  
  - **Byte offset** (*Offset*) – the position from which the receiver resumes reading.  
  - **File attributes** (*FileAttributes*) – metadata such as the file name.  

#### Filelog Receiver Batching

The Filelog Receiver does not forward log lines to the next consumer one by one. Instead, it batches them by resource. The batch size and send interval are fixed and cannot be configured—logs are sent in batches of 100 lines or every 100 milliseconds, whichever comes first.
More info about internal details can be found [here](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/31074#issuecomment-2360284799).

#### Batch Processor

The Batch Processor accepts logs and places them into batches. Batching helps better compress the data and reduce the number of outgoing connections required to transmit the data. However, there are some problems:
* The Batch Processor asynchronously handles the incoming requests and does not propagate errors to the Filelog Receiver
* The Batch Processor doesn’t preserve its state in permanent storage, once the collector exits unexpectedly, the accumulated requests are lost. 

#### Exporter Batcher

Exporter batcher is a new (not yet delivered) feature that is meant to solve the limitations of the existing batch processor. It doesn’t introduce any asynchronous behavior itself but relies on the queue sender in front of it if needed. The most important is that by using a persistent queue, no data will be lost during the shutdown.

#### Exporter Persistent Sending Queue

When persistent queue is enabled, the batches are being buffered using the provided storage extension - filestorage is a popular and safe choice. If the collector instance is killed while having some items in the persistent queue, on restart the items will be picked and the exporting is continued.
