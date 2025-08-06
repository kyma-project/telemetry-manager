
# Pipelines

To collect and export telemetry data from your Kyma cluster, define one or more pipelines for each signal type (logs, traces, metrics). You choose which data you want to collect and to which backend it's sent, as well as your preferred authentication method.

## Structure

All three pipelines are defined using Kubernetes CRDs. A CRD extends the Kubernetes API, enabling you to define new object types. In these cases, LogPipeline, TracePipeline, and MetricPipeline are the new object types you are working with.

The pipelines use [OTLP](OpenTelemetry Protocol) as the primary method for ingesting and exporting data. OTLP is a standard for sending telemetry data to various backends, offering flexibility in your monitoring setup.

You specify the kind of pipeline and to which backend the data is sent. In the input section, define where the data is collected from.

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: <Kind>Pipeline     # Pipeline kind depending on signal type
metadata:
  name: backend
spec:
  input:                 # Enable additional inputs depending on signal type
    otlp:
      ...
  output:
    otlp:                # Reference your telemetry backend
      endpoint:
      ...
```

## Input

The [`otlp`](./otlp-input.md) input of a pipeline is enabled by default and will enable a cluster internal endpoint accepting OTLP data. For more details, see [`otlp` input](./otlp-input.md).

Besides the OTLP input, different inputs are avaialble dependent on the pipeline type:

- LogPipeline: the [`application`](./../logs/application-input.md) input collects logs from your application containers' standard output (stdout) and standard error (stderr). It parses these logs, extracts useful information and transforms them into the OTLP format. Additional, Istio access log integration is available via the regular [`otlp`](./otlp-input.md) input.
- TracePipeline: No dedicated inputs are available, however Istio trace integration is available via the regular [`otlp`](./otlp-input.md) input.
- MetricPipeline: The [`prometheus`](./../metrics/prometheus-input.md) input enables a pull-based metric collection in the Prometheus format using an annotation-based discovery. The [`runtime`](./../metrics/runtime-input.md) input supports the collection of Kubernetes runtime metrics (things like CPU/memory usage). You can also configure the collection of [`istio`](./../metrics/istio-input.md) proxy metrics, providing detailed information about service mesh traffic.

## Output

All pipelines support only the [`otlp`](./otlp-input.md) output (besides the [legacy LogPipeline](./../02-logs.md) feature) for exporting data. For more details, see [`otlp` output](./otlp-output.md).

## Data Enrichment

All pipelines are performing advanced enrichment of the data leveraging the OTel resource attributes. With that the sources of the data can be identified with ease in the backends. For more details, see [Enrichment](./enrichment.md).

## Troubleshooting

For more details, see [Troubleshooting](./troubleshooting.md)

## Operations

For more details, see [Operations](./operations.md)
