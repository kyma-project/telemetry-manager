# Collect Prometheus Metrics

If your applications emit Prometheus-based metrics, enable the `prometheus` input in your `MetricPipeline` and annotate your application's Service or Pod. You can collect diagnostic metrics and control from which namespaces the system collects metrics.

## Prerequisites

Instrument your application using a library like the [Prometheus client library](https://prometheus.io/docs/instrumenting/clientlibs/) or the OTel SDK (with [Prometheus exporter](https://opentelemetry.io/docs/specs/otel/metrics/sdk_exporters/prometheus/)). Expose a port in your workload as a Prometheus metrics endpoint.

## Activate Prometheus Metrics

The `prometheus` input is disabled by default. If your applications emit Prometheus metrics, enable Prometheus-based metric collection:

```yaml
  ...
  input:
    prometheus:
      enabled: true
```

> [!TIP]
> To validate or debug your configuration, use diagnostic metrics (see [Collect Diagnostic Metrics](#collect-diagnostic-metrics)).

## Enable Metrics Collection With Annotations

The metric agent automatically discovers Prometheus endpoints in your cluster. It looks for specific annotations on your Kubernetes Services or Pods.

Apply the following annotations to enable automatic metric collection. If your Pod has an Istio sidecar, annotate the Service. Otherwise, annotate the Pod directly.

> **Note:** If your service mesh enforces `STRICT` mTLS, the agent scrapes the endpoint over HTTPS automatically. If you do not use `STRICT` mTLS, add the `prometheus.io/scheme: http` annotation to force scraping over plain HTTP.

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

## Scrape Metrics from Istio-enabled Workloads

If your application is part of an Istio service mesh, you must consider service port naming and mutual TLS (mTLS) configuration:

- Istio must be able to identify the `appProtocol` from the Service port definition; otherwise Istio may block the scrape request.
  You must either prefix the port name with the protocol like in `http-metrics`, or explicitly define the `appProtocol` attribute.

- The metric agent can scrape endpoints from workloads that enforce mutual TLS (mTLS). For scraping through HTTPS, Istio must configure the workload using STRICT mTLS mode.
  If you can't use STRICT mTLS mode, you can set up scraping through plain HTTP by adding the following annotation to your Service: `prometheus.io/scheme: http`. For related troubleshooting, see [Log Entry: Failed to Scrape Prometheus Endpoint](../troubleshooting.md#metricpipeline-failed-to-scrape-prometheus-endpoint).

## Collect Diagnostic Metrics
<!-- identical section for Prometheus and Istio docs -->
To validate or debug your scraping configuration for the `prometheus` and `istio` input, you can use diagnostic metrics. By default, they are disabled.

> **Note:** Unlike the `prometheus` and `istio` inputs, the `runtime`  input gathers data directly from Kubernetes APIs instead of using a scraping process, so it does not generate scrape-specific diagnostic metrics.

To use diagnostic metrics, enable the `diagnosticMetrics` for the input in your MetricPipeline:

```yaml
  ...
  input:
    istio:
    enabled: true
    diagnosticMetrics:
        enabled: true
```

When enabled, the metric agent generates metrics about its own scrape jobs, such as the following:

- `scrape_duration_seconds`
- `scrape_samples_scraped`
- `scrape_samples_post_metric_relabeling`
- `scrape_series_added`
