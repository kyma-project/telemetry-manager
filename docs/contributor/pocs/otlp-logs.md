# OpenTelemetry Logs PoC

This PoC researches two main aspects of OpenTelemetry logs: [Log Record Parsing](#log-record-parsing) and [Buffering and Backpressure](#buffering-and-backpressure).

## Log Record Parsing

### Scope and Goals

When integrating an OTLP-compliant logging backend, applications can either ingest their logs directly or emit them to STDOUT and use a log collector to process and forward the logs.
The first part of this PoC evaluates how to configure the OpenTelemetry Collector's [filelog receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/filelogreceiver) to transform structured JSON logs emitted by Kubernetes workloads to STDOUT, and subsequently to the [OTLP logs data model](https://opentelemetry.io/docs/specs/otel/logs/data-model/).
OpenTelemtry Collector should move JSON attributes to the **attributes** map of the log record, extract other fields like **severity** or **timestamp**, write the actual log message to the **body** field, and add any missing information to ensure that the **attributes** and **resource** attributes comply with the semantic conventions.

This PoC does not cover logs ingested by the application using the OTLP protocol. We assume that the application already fills the log record fields with the intended values.

## Setup for Log Record Parsing

We created a Helm values file for the `open-telemetry/opentelemetry-collector` chart that parses and transforms container logs in the described way. We use an SAP Cloud Logging instance as the OTLP-compliant logging backend. To deploy the setup, follow these steps:

1. Create an SAP Cloud Logging instance. Store the endpoint, client certificate, and key under the keys `ingest-otlp-endpoint`, `ingest-otlp-cert`, and `ingest-otlp-key` respectively, in a Kubernetes Secret within the `otel-logging` namespace.

2. Deploy the OpenTelemetry Collector Helm chart with the values file [otlp-logs.yaml](assets/otel-logs-values.yaml):

   ```bash
   helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
   helm install -n otel-logging logging open-telemetry/opentelemetry-collector -f ./assets/otel-logs-values.yaml
   ```

### Results

We tested different log formats to evaluate the filelog receiver configuration. The following example of a log record emitted by telemetry-metric-agent demonstrates the transformation. The original log record looks as follows:

```
{"level":"info","ts":1706781583.437593,"caller":"exporterhelper/retry_sender.go:129","msg":"Exporting failed. Will retry the request after interval.","kind":"exporter","data_type":"metrics","name":"otlp","error":"rpc error: code = Unavailable desc = no healthy upstream","interval":"6.132976949s"}
```

This processed log record arrives in the SAP Cloud Logging (OpenSearch):

```
{
  "_index": "logs-otel-v1-2024.02.01",
  "_type": "_doc",
  "_id": "20ccZI0BYhUzrpscNwrE",
  "_version": 1,
  "_score": null,
  "_source": {
    "traceId": "",
    "spanId": "",
    "severityText": "info",
    "flags": 0,
    "time": "2024-02-01T09:59:43.437812119Z",
    "severityNumber": 9,
    "droppedAttributesCount": 0,
    "serviceName": null,
    "body": "Exporting failed. Will retry the request after interval.",
    "observedTime": "2024-02-01T09:59:43.580359394Z",
    "schemaUrl": "",
    "log.attributes.time": "2024-02-01T09:59:43.437812119Z",
    "log.attributes.original": "{\"level\":\"info\",\"ts\":1706781583.437593,\"caller\":\"exporterhelper/retry_sender.go:129\",\"msg\":\"Exporting failed. Will retry the request after interval.\",\"kind\":\"exporter\",\"data_type\":\"metrics\",\"name\":\"otlp\",\"error\":\"rpc error: code = Unavailable desc = no healthy upstream\",\"interval\":\"6.132976949s\"}",
    "resource.attributes.k8s@namespace@name": "kyma-system",
    "resource.attributes.k8s@container@name": "collector",
    "resource.attributes.security@istio@io/tlsMode": "istio",
    "log.attributes.log@iostream": "stderr",
    "log.attributes.name": "otlp",
    "resource.attributes.k8s@pod@name": "telemetry-metric-agent-8wxcx",
    "resource.attributes.k8s@node@name": "...",
    "resource.attributes.service@istio@io/canonical-name": "telemetry-metric-agent",
    "resource.attributes.service@istio@io/canonical-revision": "latest",
    "resource.attributes.app@kubernetes@io/name": "telemetry-metric-agent",
    "log.attributes.level": "info",
    "resource.attributes.k8s@daemonset@name": "telemetry-metric-agent",
    "log.attributes.logtag": "F",
    "log.attributes.data_type": "metrics",
    "resource.attributes.k8s@pod@start_time": "2024-02-01 09:59:25 +0000 UTC",
    "resource.attributes.controller-revision-hash": "7758d58497",
    "log.attributes.error": "rpc error: code = Unavailable desc = no healthy upstream",
    "resource.attributes.pod-template-generation": "2",
    "log.attributes.log@file@path": "/var/log/pods/kyma-system_telemetry-metric-agent-8wxcx_a01b36e5-28a0-4e31-9ee5-615ceed08321/collector/0.log",
    "resource.attributes.k8s@pod@uid": "a01b36e5-28a0-4e31-9ee5-615ceed08321",
    "resource.attributes.sidecar@istio@io/inject": "true",
    "log.attributes.ts": 1706781583.437593,
    "log.attributes.kind": "exporter",
    "resource.attributes.k8s@container@restart_count": "0",
    "log.attributes.interval": "6.132976949s",
    "log.attributes.caller": "exporterhelper/retry_sender.go:129"
  },
  "fields": {
    "observedTime": [
      "2024-02-01T09:59:43.580Z"
    ],
    "time": [
      "2024-02-01T09:59:43.437Z"
    ]
  },
  "sort": [
    1706781583437
  ]
}
```

The OpenTelemetry Collector configuration moves all JSON fields to the **attributes** map. The user-given log message emitted in the **msg** JSON field is moved to the OTLP **body** field.
The **level** JSON field determines the **severityName** and **severityNumber** fields. The mapping is automatically performed using the severity_parser operator.
Operators for the filelog receiver determine the emitting Pod. The k8sattributes processor adds other resource attributes to fulfill the semantic conventions.
The k8sattributes processor is also used to create resource attributes for pod labels. The same could be done with annotations.
An operator for the filelog receiver preserves the originating filesystem path of the record to be compliant with the semantic conventions for logs.
In the used configuration, we move the original log record to the **original** attribute for debugging purposes.

The OpenTelemetry Collector setup is able to extract the log message from different attributes, depending on their presence. This means that it is possible to support different logging libraries.

Non-JSON logs are preserved in the **body** field until the enrichment with resource attributes is completed.

## Buffering and Backpressure

### Scope and Goals

After evaluating the filelog receiver configuration in [Log Record Parsing](#log-record-parsing), we want to test the buffering and backpressure capabilities of the OpenTelemetry Collector. The OpenTelemetry-based logging solution should give similar resilience and guarantees about log delivery as the current logging solution.

## Setup for Buffering and Backpressure

We split the OpenTelemetry Collector for log processing to an agent (DaemonSet) and a gateway (StatefulSet). The agent uses the same configuration as shown in [Log Record Parsing](#log-record-parsing) to read logs from the host file system, and converts them to the OTLP format, while the gateway adds Kubernetes metadata and ensures that no logs are lost if the backend fails.

The following figure shows the different plugins that are configured in the processing pipeline. Note the use of the batch processor in the gateway, which introduces asynchronicity to the pipeline and causes that backpressure is not propagated back to the agent. To minimize the risk of log loss due to the batch processors properties, a persistent exporter queue was introduced in the gateway, which uses a persistent volume to buffer logs in case of a backend failure.

![Otel Collector Setup](./assets/otlp-logs.drawio.svg)

To deploy the OpenTelemetry Collector agent and gateway, perform the following steps:

1. Create a SAP Cloud Logging instance as described in [Setup for Log Record Parsing](#setup-for-log-record-parsing).

1. Create a persistent volume claim (PVC):

   ```bash
   kubectl apply -n otel-logging -f ./assets/otel-gateway-pvc.yaml
   ```

1. Deploy the gateway:

   ```bash
   helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
   helm install -n otel-logging logging open-telemetry/opentelemetry-collector -f ./assets/otel-log-gateway-values.yaml
   ```

1. Deploy the agent:

   ```bash
   helm install -n otel-logging logging open-telemetry/opentelemetry-collector -f ./assets/otel-log-agent-values.yaml
   ```

### Evaluation

To evaluate the buffering and backpressure capabilities of the described OpenTelemetry Collector setup, we tested the following scenarios and observed the respective behavior:

* **Outage of the OTLP backend**

  Log records cannot be shipped from the gateway to the OTLP backend (SAP Cloud Logging). When the configured queue limit is reached, log records are dropped from the queue. The enqueue errors are not propagated back to other pipeline elements because of the asynchronicity introduced by the batch processor.

* **Broken connectivity between the agent and the gateway**

  Log records cannot be exported by the agent to the gateway using the OTLP protocol. The exporter queue on the agent buffers up to its maximum size and then starts rejecting new records. This enqueue error is propagated to the filelog receiver, which eventually stops reading new logs. Log loss is avoided until the log retention of the kubelet removes old logs.

### Conclusions

The evaluation of the two failure scenarios shows that the OpenTelemetry Collector can have similar guarantees about the prevention of log loss as the current Fluent Bit setup. When using a batch processor, we can prevent data loss with a persistent output queue, which increases the queue capacity. By splitting that processing pipeline to agent and gateway, we can use a PVC for the exporter queue, which provides large capacity without the risk that the node file system fills up.

During the evaluation, the following potential problems and risks have been identified:

* The persistent queue of the OpenTelemetry Collector is still in alpha state and might not be suitable yet for production use.
* The queue capacity is configured by the number of batches. A limitation based on storage capacity is impossible. This makes it hard to give exact guarantees about the stored logs before data loss.
* After it is allocated, the utilized storage space of the persistent queue never shrinks again. This is not a problem as long as a dedicated PVC is used for the queue, but makes it less suitable to be stored on the node's host file system.
* Not using a batch processor in the agent might have a negative performance impact.
