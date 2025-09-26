# Logs Application Input

To enable collection of logs printed by containers to the `stdout/stderr` channel, define a LogPipeline that has the `application` section enabled as input:

```yaml
  ...
  input:
    application:
      enabled: true
```

By default, input is collected from all namespaces, except the system namespaces `kube-system`, `istio-system`, `kyma-system`, which are excluded by default.

## Namespace and Container Filter

To filter your application logs by namespace or container, use an input spec to restrict or specify which resources you want to include. For example, you can define the namespaces to include in the input collection, exclude namespaces from the input collection, or choose that only system namespaces are included. Learn more about the available [parameters and attributes](./../resources/02-logpipeline.md).

The following pipeline collects input from all namespaces excluding `kyma-system` and only from the `istio-proxy` containers:

```yaml
  ...
  input:
    application:
      enabled: true
      keepOriginalBody: true
      namespaces:
        exclude:
          - myNamespace
      containers:
        exclude:
          - myContainer
```

After tailing the log files from the container runtime, the payload of the log lines is transformed into an OTLP entry. Learn more about the flow of the log record through the steps and the available log attributes in the following stages:

- [Log Tailing](#log-tailing)
- [JSON Parsing](#json-parsing)
- [Severity Parsing](#severity-parsing)
- [Trace Parsing](#trace-parsing)
- [Log Body Determination](#log-body-determination)

The following example assumes that there’s a container `myContainer` of Pod `myPod`, running in namespace `myNamespace`, logging to `stdout` with the following log message in the JSON format:

```json
{
  "level": "warn",
  "message": "This is the actual message",
  "tenant": "myTenant",
  "traceID": "123"
}
```

## Log Tailing

The agent reads the log message from a log file managed by the container runtime. The file name contains namespace, Pod and Container information that will be available later as log attributes. The raw log record looks similar to the following example:

```json
{
  "time": "2022-05-23T15:04:52.193317532Z",
  "stream": "stdout",
  "_p": "F",
  "log": "{\"level\": \"warn\",\"message\": \"This is the actual message\",\"tenant\": \"myTenant\",\"trace_id\": \"123\"}"
}
```

After the tailing, the created OTLP record looks like the following example:

```json
{
  "time": "2022-05-23T15:04:52.100000000Z",
  "observedTime": "2022-05-23T15:04:52.200000000",
  "attributes": {
   "log.file.path": "/var/log/pods/myNamespace_myPod-<containerID>/myContainer/<containerRestarts>.log",
   "log.iostream": "stdout"
  },
  "resourceAttributes": {
    "k8s.container.name": "myContainer",
    "k8s.container.restart_count": "<containerRestarts>",
    "k8s.pod.name": "myPod",
    "k8s.namespace.name": "myNamespace"
  },
  "body": "{\"level\": \"warn\",\"message\": \"This is the actual message\",\"tenant\": \"myTenant\",\"trace_id\": \"123\"}"
}
```

All information identifying the source of the log (like the Container, Pod and namespace name) are enriched as resource attributes following the [Kubernetes conventions](https://opentelemetry.io/docs/specs/semconv/resource/k8s/). Further metadata - like the original file name and channel - are enriched as log attributes following the [log attribute conventions](https://opentelemetry.io/docs/specs/semconv/general/logs/). The **time** value provided in the container runtime log entry is used as **time** attribute in the new OTel record, as it is very close to the actual time when the log happened. Additionally, the **observedTime** is set with the time when the agent actual read the log record as recommended by the [OTel log specification](https://opentelemetry.io/docs/specs/otel/logs/data-model/#field-observedtimestamp). The log payload is moved to the OTLP **body** field.

## JSON Parsing

If the value of the **body** is a JSON document, the value is parsed and all JSON root attributes are enriched as additional log attributes. The original body is moved into the **log.original** attribute (managed with the LogPipeline attribute **input.application.keepOriginalBody**: `true`).

The resulting OTLP record looks like the following example:

```json
{
  "time": "2022-05-23T15:04:52.100000000Z",
  "observedTime": "2022-05-23T15:04:52.200000000",
  "attributes": {
   "log.file.path": "/var/log/pods/myNamespace_myPod-<containerID>/myContainer/<containerRestarts>.log",
   "log.iostream": "stdout",
   "log.original": "{\"level\": \"warn\",\"message\": \"This is the actual message\",\"tenant\": \"myTenant\",\"trace_id\": \"123\"}",
   "level": "warn",
   "tenant": "myTenant",
   "trace_id": "123",
   "message": "This is the actual message"
  },
  "resourceAttributes": {
    "k8s.container.name": "myContainer",
    "k8s.container.restart_count": "<containerRestarts>",
    "k8s.pod.name": "myPod",
    "k8s.namespace.name": "myNamespace"
  },
  "body": ""
}
```

## Severity Parsing

Typically, a log message has a log level written to a field `level`. Based on that, the agent tries to parse the log attribute **level** with a [severity parser](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/stanza/docs/operators/severity_parser.md). If that is successful, the log attribute is transformed into the OTel attributes **severityText** and **severityNumber**.

## Trace Parsing

OTLP natively supports attaching trace context to log records. If possible, the log agent parses the following log attributes according to the [W3C-Tracecontext specification](https://www.w3.org/TR/trace-context/#traceparent-header):

- **trace_id**
- **span_id**
- **trace_flags**
- **traceparent**

## Log Body Determination

Because the actual log message is typically located in the **body** attribute, the agent moves a log attribute called **message** (or **msg**) into the **body**.

At this point, before further enrichment, the resulting overall log record looks like the following example:

```json
{
  "time": "2022-05-23T15:04:52.100000000Z",
  "observedTime": "2022-05-23T15:04:52.200000000",
  "attributes": {
   "log.file.path": "/var/log/pods/myNamespace_myPod-<containerID>/myContainer/<containerRestarts>.log",
   "log.iostream": "stdout",
   "log.original": "{\"level\": \"warn\",\"message\": \"This is the actual message\",\"tenant\": \"myTenant\",\"trace_id\": \"123\"}",
   "tenant": "myTenant",
  },
  "resourceAttributes": {
    "k8s.container.name": "myContainer",
    "k8s.container.restart_count": "<containerRestarts>",
    "k8s.pod.name": "myPod",
    "k8s.namespace.name": "myNamespace"
  },
  "body": "This is the actual message",
  "severityNumber": 13,
  "severityTex": "warn",
  "trace_id": 123
}
```
