# OpenTelemetry Logs PoC

## Scope and Goals

When integrating an OTLP compliant logging backend, applications can either ingest their logs directly or emit them to STDOUT and have a log collector to process logs and forward them.
With this PoC, we evaluated how the [filelog receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/filelogreceiver) of the OpenTelemetry Collector can be configured to transform structured JSON logs that are emitted by Kubernetes workloads to STDOUT to the [OTLP logs data model](https://opentelemetry.io/docs/specs/otel/logs/data-model/).
The OpenTelemtry Collector should move JSON attributes to the `attributes` map of the log record, extract other fields like the severity, timestamp, write the actual log message to the `body` field, and add missing information to be compliant with the semantic conventions for `attributes` and `resource` attributes.

Logs that are ingested by the application using the OTLP protocol are not covered by this PoC. We assume that the application is already filling the log record fields with the intended values.

## Setup

We created a Helm values file for the open-telemetry/opentelemetry-collector chart that parses and transforms container logs in the described way. We use an SAP Cloud Logging instance as the OTLP compliant logging backend. To deploy the setup, follow these steps:

1. Create a SAP Cloud Logging instance and have the endpoint, client certificate and key under the keys `ingest-otlp-endpoint`, `ingest-otlp-cert` and `ingest-otlp-key` in a Kubernetes secret in the `otel-logging` namespace. This can be done using a ServiceBinding with the BTP Operator.

2. Deploy the OpenTelemetry Collector Helm chart with the values file [otlp-logs.yaml](../assets/otel-logs-values.yaml):

   ```bash
   helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
   helm install -n otel-logging logging open-telemetry/opentelemetry-collector \
     -f ../assets/otel-logs-values.yaml
   ```

## Results

We tested different log formats to evaluate the filelog receiver configuration. The following example of a log record that was emitted by the telemetry-metric-agent demonstrates the transformation. The original log record looks as follows:

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

The OpenTelemetry Collector configuration moves all JSON fields to the `attribtues` map. The user given log message, which emitted in the `msg` JSON field, is moved to the OTLP `body` field.
The `level` JSON field is used to determine `severityName` and `severityNumber` fields. The mapping is done automatically using the severity_parser operator.
Operators for the filelog receiver determine the emitting Pod. Other resource attributes are added by the k8sattributes processor to fulfill the semantic conventions.
The k8sattributes processor is also used to create resource attributes for pod labels. The same could be done with annotations.
An operator for the filelog receiver preserves the originating filesystem path of the record to be compliant with the semantic conventions for logs.
In the used configuration, we move the original log record to the `original` attribute for debugging purposes.

The OpenTelemetry Collector setup is able to extract the log message from different attributes, based on their existence. I.e., it is possible to support different logging libraries.

Non-JSON logs are for now preserved in the Body field while the enchrichment with resource attributes is still done.