# Metrics Prometheus Input

To enable collection of Prometheus-based metrics, define a MetricPipeline that has the `prometheus` section enabled as input:

```yaml
  ...
  input:
    prometheus:
      enabled: true
```

> [!NOTE]
> For the following approach, you must have instrumented your application using a library like the [Prometheus client library](https://prometheus.io/docs/instrumenting/clientlibs/) or the [OTel SDK having the Prometheus exporter configured](https://opentelemetry.io/docs/specs/otel/metrics/sdk_exporters/prometheus/), with a port in your workload exposed serving as a Prometheus metrics endpoint.

## Endpoint Discovery

The Metric agent is configured with a generic scrape configuration, which uses annotations to discover the endpoints to scrape in the cluster.

For metrics ingestion to start automatically, use the annotations of the following table.
If an Istio sidecar is present, apply them to a Service that resolves your metrics port.
By annotating the Service, all endpoints targeted by the Service are resolved and scraped by the Metric agent bypassing the Service itself.
Only if Istio sidecar is not present, you can alternatively apply the annotations directly to the Pod.

| Annotation Key | Example Values | Default Value | Description |
|--|--|--|--|
| `prometheus.io/scrape` (mandatory) | `true`, `false` | none | Controls whether Prometheus Receiver automatically scrapes metrics from this target. |
| `prometheus.io/port` (mandatory) | `8080`, `9100` | none | Specifies the port of the Pod where the metrics are exposed. |
| `prometheus.io/path` | `/metrics`, `/custom_metrics` | `/metrics` | Defines the HTTP path where Prometheus Receiver can find metrics data. |
| `prometheus.io/scheme` (only relevant when annotating a Service) | `http`, `https` | If Istio is active, `https` is supported; otherwise, only `http` is available. The default scheme is `http` unless an Istio sidecar is present, denoted by the label `security.istio.io/tlsMode=istio`, in which case `https` becomes the default. | Determines the protocol used for scraping metrics — either HTTPS with mTLS or plain HTTP. |
| `prometheus.io/param_<name>: <value>` | `prometheus.io/param_format: prometheus` | none | Instructs Prometheus Receiver to pass name-value pairs as URL parameters when calling the metrics endpoint. |

If you're running the Pod targeted by a Service with Istio, Istio must be able to derive the [appProtocol](https://kubernetes.io/docs/concepts/services-networking/service/#application-protocol) from the Service port definition; otherwise the communication for scraping the metric endpoint cannot be established. You must either prefix the port name with the protocol like in `http-metrics`, or explicitly define the `appProtocol` attribute.

For example, see the following `Service` configuration:

```yaml
apiVersion: v1
kind: Service
metadata:
  annotations:
    prometheus.io/port: "8080"
    prometheus.io/scrape: "true"
  name: sample
spec:
  ports:
  - name: http-metrics
    appProtocol: http
    port: 8080
    protocol: TCP
    targetPort: 8080
  selector:
    app: sample
  type: ClusterIP
```

> [!NOTE]
> The Metric agent can scrape endpoints even if the workload is a part of the Istio service mesh and accepts mTLS communication. However, there's a constraint: For scraping through HTTPS, Istio must configure the workload using 'STRICT' mTLS mode. Without 'STRICT' mTLS mode, you can set up scraping through HTTP by applying the annotation `prometheus.io/scheme=http`. For related troubleshooting, see [Log Entry: Failed to Scrape Prometheus Endpoint](./README.md#log-entry-failed-to-scrape-prometheus-endpoint).

## Filters

To filter metrics by namespaces, define a MetricPipeline that has the `namespaces` section defined in one of the inputs. For example, you can specify the namespaces from which metrics are collected or the namespaces from which metrics are dropped. Learn more about the available [parameters and attributes](./../resources/05-metricpipeline.md).

By default, the sidecars of all namespaces are getting collected excluding system namespaces. To include system namespaces as well, please explicitly configure an empty namespcae object: `namespaces: {}`.

The following example collects runtime metrics **only** from the `foo` and `bar` namespaces:

```yaml
  ...
  input:
    prometheus:
      enabled: true
      namespaces:
        include:
          - foo
          - bar
```

The following example collects runtime metrics from all namespaces **except** the `foo` and `bar` namespaces:

```yaml
  ...
  input:
    prometheus:
      enabled: true
      namespaces:
        exclude:
          - foo
          - bar
```

## Diagnostic Metrics

When the metric agent is scraping metrics, it instruments operational metrics for every source of a metric, such as `up`, `scrape_duration_seconds`, `scrape_samples_scraped`, `scrape_samples_post_metric_relabeling`, and `scrape_series_added`.

By default, these operational metrics are disabled.

If you want to use them for debugging and diagnostic purposes, you can activate them. To activate diagnostic metrics, define a MetricPipeline that has the `diagnosticMetrics` section defined.

The following example enables diagnostic metrics:

```yaml
  ...
  input:
    istio:
    enabled: true
    diagnosticMetrics:
        enabled: true
```
