# 11. Impact of Istio Prometheus Merge Feature on MetricPipelines

Date: 2024-10-21

## Status

Approved

## Context

The MetricPipeline supports various input types, including `prometheus` and `istio`.
* When the `prometheus` input is enabled, it scrapes metrics from Pods and Services annotated with "prometheus.io/scrape=true".
* When the `istio` input is enabled, it scrapes metrics from the Istio proxy (sidecar) containers injected into Pods.

Both inputs are technically backed by the OpenTelemetry Collector’s Prometheus Receiver. There are five distinct scrape jobs, each targeting different configurations:

| Scrape Job | Targets |
| --- | --- |
| app-pods | Discovers annotated Pods that either do not have an Istio sidecar or have the "prometheus.io/scheme" annotation set to "http". Scraping is done over plain HTTP. |
| app-services | Discovers annotated Services backed by Pods without an Istio sidecar or have the "prometheus.io/scheme" Service annotation set to "http". Scraping is done over plain HTTP. |
| app-pods-secure | Discovers annotated Pods that either have an Istio sidecar or have the "prometheus.io/scheme" annotation set to "https". Scraping is performed over HTTPS, using a client TLS certificate injected by Istio. |
| app-services-secure | Discovers annotated Services backed by Pods with an Istio sidecar or where the "prometheus.io/scheme" Service annotation is set to "https". Scraping is done over HTTPS, using a client TLS certificate injected by Istio. |
| istio-proxy | Discovers metrics from Istio proxy sidecars. Scraping is done over plain HTTP. |

In general, annotating Services offers much more flexibility than annotating Pods due to the following reasons:
* A Pod can only have a single "prometheus.io/port" annotation, meaning that if multiple containers are running in the Pod (or single container exposes multiple ports), only one can be scraped. With Services, multiple Services can be created to target different ports, allowing all relevant metrics to be scraped.
* For istiofied Pods, even with annotations, a Service is still required to define the application protocol in order for scraping to occur. The Service itself doesn’t need annotations but must be present to enable scraping. See more: https://kyma-project.io/#/telemetry-manager/user/04-metrics?id=_4-activate-prometheus-based-metrics.

As discussed in [issue #1468](https://github.com/kyma-project/telemetry-manager/issues/1468), the `prometheusMerge` feature is crucial for simplifying Dynatrace integration and enhancing security. For the sake of explanation, let’s assume that `prometheusMerge` is always enabled in the Istio mesh configuration. Under this scenario, the following points are true:

* Istio sidecars will merge Istio’s metrics with the application’s metrics. The combined metrics will be exposed at `:15020/stats/prometheus` for scraping using plain HTTP.
No HTTPS scraping is possible anymore.
* The necessary "prometheus.io" annotations will be added to all data plane Pods to configure scraping. If these annotations already exist, they will be overwritten. It will make impossible to distinguish between Pods that expose application metrics and those that don't.
* It is hard to reliably separate the merged metrics into distinct Istio and application metrics, aside from maintaining a hard-coded list of Istio proxy metrics. As a result, it is impossible to isolate `prometheus` and `istio` inputs.

## Decision

Discontinue support for scraping annotated Pods that have Istio sidecars, limiting support to annotated Pods without Istio sidecars. This change ensures that the prometheusIstio feature remains independent from the MetricPipeline feature set, so that it can be enabled without impacting other functionality. The implications are as follows:
* The app-pods-secure scrape job will be removed.
* The app-pods scrape job will be updated to exclude targets with Istio sidecars, using a marker label (as is currently implemented).
* The `prometheusMerge` feature can be enabled by default since it has no impact on the MetricPipeline functionality.

Annotating Services is the preferred method over annotating Pods. However, we will continue to support Pod annotations for non-Istio workloads to accommodate users installing off-the-shelf software (e.g., via Helm), which often includes annotated Pods because of legacy reasons. For custom workloads, Services should be promoted as the recommended approach for annotation.
