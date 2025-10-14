# Collect Prometheus Metrics

To collect metrics from applications that expose a Prometheus-compatible endpoint, enable the **prometheus** input in your MetricPipeline and annotate your Pods or Services for discovery. You can enable diagnostic metrics and control from which namespaces metrics are collected.

## Prerequisites

Instrument your application using a library like the [Prometheus client library](https://prometheus.io/docs/instrumenting/clientlibs/) or the OTel SDK (with [Prometheus exporter](https://opentelemetry.io/docs/specs/otel/metrics/sdk_exporters/prometheus/)). Expose a port in your workload as a Prometheus metrics endpoint.

## Activate Prometheus Metrics

By default, the **prometheus** input is disabled. If your applications emit Prometheus metrics, enable the collection of Prometheus-based metrics:

```yaml
  ...
  input:
    prometheus:
      enabled: true
```

> [!TIP]
> To validate or debug your configuration, use diagnostic metrics (see [Collect Diagnostic Metrics](#collect-diagnostic-metrics)).

## Enable Metrics Collection With Annotations

The metric agent automatically discovers Prometheus endpoints in your cluster by looking for specific annotations on your Kubernetes Services or Pods.

To enable automatic metrics collection, apply the following annotations. If your Pod has an Istio sidecar, annotate the Service. Otherwise, annotate the Pod directly.

> **Note:** If your service mesh enforces `STRICT` mTLS, the agent scrapes the endpoint over HTTPS automatically. If you don't use `STRICT` mTLS, add the annotation `prometheus.io/scheme: http` to force scraping over plain HTTP.

| Annotation Key                   | Values                                                           | Description                                                                                                                        |
|----------------------------------|------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------|
| `prometheus.io/scrape` (mandatory) | true, false (no default value)                                   | Set to true to enable scraping for this target.                                                                                    |
| `prometheus.io/port` (mandatory)   | 8080, 9100 (no default value)                                    | Specify the port on the Pod where your application exposes metrics.                                                                |
| `prometheus.io/path`               | /metrics (default), /custom_metrics                              | Set the HTTP path for the metrics endpoint.                                                                                        |
| `prometheus.io/scheme`             | https-metrics (default with Istio), http (default without Istio) | Define the protocol for scraping: Either HTTPS with mTLS, or plain HTTP.                                                           |
| `prometheus.io/param_<name>`       | Example: format: prometheus (no default)                         | Add a URL parameter to the scrape request. For example, prometheus.io/param_format: prometheus adds ?format=prometheus to the URL. |

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

- Istio must be able to identify the **appProtocol** from the Service port definition; otherwise Istio may block the scrape request.
  You must either prefix the port name with the protocol like in `http-metrics`, or explicitly define the **appProtocol** attribute.

- The metric agent can scrape endpoints from workloads that enforce mutual TLS (mTLS). For scraping through HTTPS, Istio must configure the workload using STRICT mTLS mode.
  If you can't use STRICT mTLS mode, you can set up scraping through plain HTTP by adding the following annotation to your Service: `prometheus.io/scheme: http`. For related troubleshooting, see [MetricPipeline: Failed to Scrape Prometheus Endpoint](../troubleshooting.md#metricpipeline-failed-to-scrape-prometheus-endpoint).

## Collect Diagnostic Metrics
<!-- identical section for Prometheus and Istio docs -->
To validate or debug your scraping configuration for the **prometheus** and **istio** input, you can use diagnostic metrics. By default, they are disabled.

> **Note:** Unlike the **prometheus** and **istio** inputs, the **runtime**  input gathers data directly from Kubernetes APIs instead of using a scraping process, so it does not generate scrape-specific diagnostic metrics.

To use diagnostic metrics, enable the **diagnosticMetrics** for the input in your MetricPipeline:

```yaml
  ...
  input:
    <istio | prometheus>:
    enabled: true
    diagnosticMetrics:
        enabled: true
```

When enabled, the metric agent generates metrics about its own scrape jobs, such as the following:

- `up`: The scraping was successful
- `scrape_duration_seconds`: Duration of the scrape
- `scrape_samples_scraped`: The number of samples the target exposed
- `scrape_samples_post_metric_relabeling`: The number of samples remaining after metric relabeling was applied
- `scrape_series_added`: The approximate number of new series in this scrape
