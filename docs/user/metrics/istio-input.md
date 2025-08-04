# Istio Input

To enable collection of Istio metrics, define a MetricPipeline that has the `istio` section enabled as input:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  input:
    istio:
      enabled: true
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
```

With this, the agent starts collecting all Istio metrics from Istio sidecars from all namespaces (including system namespaces).

## Filters

To filter sidecars to collect by namespaces, define a MetricPipeline that has the `namespaces` section defined in one of the inputs. For example, you can specify the namespaces from which metrics are collected or the namespaces from which metrics are dropped. Learn more about the available [parameters and attributes](resources/05-metricpipeline.md).

By default, the sidecars of all namespaces are getting collected.

The following example collects runtime metrics **only** from the `foo` and `bar` namespaces:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  input:
    istio:
      enabled: true
      namespaces:
        include:
          - foo
          - bar
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
```

The following example collects runtime metrics from all namespaces **except** the `foo` and `bar` namespaces:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  input:
    istio:
      enabled: true
      namespaces:
        exclude:
          - foo
          - bar
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
```

## Envoy metrics

If you are using the `istio` input by default only the metrics are collect starting with the prefix `istio_`. You can also collect Envoy metrics having the prefix `envoy_`. Envoy metrics provide insights into the performance and behavior of the Envoy proxy, such as request rates, latencies, and error counts. These metrics are useful for observability and troubleshooting service mesh traffic.

For details, see the list of available [Envoy metrics](https://www.envoyproxy.io/docs/envoy/latest/configuration/upstream/cluster_manager/cluster_stats) and [server metrics](https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/statistics).

> [!NOTE]
> Envoy metrics are only available for the `istio` input. Ensure that Istio sidecars are correctly injected into your workloads for Envoy metrics to be available.

By default, Envoy metrics collection is disabled.

To activate Envoy metrics, enable the `envoyMetrics` section in the MetricPipeline specification under the `istio` input:

  ```yaml
  apiVersion: telemetry.kyma-project.io/v1alpha1
  kind: MetricPipeline
  metadata:
    name: envoy-metrics
  spec:
    input:
      istio:
        enabled: true
        envoyMetrics:
          enabled: true
    output:
      otlp:
        endpoint:
          value: https://backend.example.com:4317
  ```

## Diagnostic Metrics

The agent which is scraping the metrics instruments for every metric source operational metrics, such as `up`, `scrape_duration_seconds`, `scrape_samples_scraped`, `scrape_samples_post_metric_relabeling`, and `scrape_series_added`.

By default, they are disabled.

If you want to use them for debugging and diagnostic purposes, you can activate them. To activate diagnostic metrics, define a MetricPipeline that has the `diagnosticMetrics` section defined.

The following example enables diagnostic metrics:

  ```yaml
  apiVersion: telemetry.kyma-project.io/v1alpha1
  kind: MetricPipeline
  metadata:
    name: backend
  spec:
    input:
      istio:
        enabled: true
        diagnosticMetrics:
          enabled: true
    output:
      otlp:
        endpoint:
          value: https://backend.example.com:4317
  ```
