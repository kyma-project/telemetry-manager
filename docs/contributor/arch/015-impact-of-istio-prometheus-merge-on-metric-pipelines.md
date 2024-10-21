# 11. Impact of Istio Prometheus Merge Feature on MetricPipelines

Date: 2024-10-21

## Status

Proposed

## Context

The MetricPipeline supports various input types, including prometheus and istio.
* When the prometheus input is enabled, it scrapes metrics from Pods and Services annotated with prometheus.io/scrape=true.
* When the istio input is enabled, it scrapes metrics from the Istio proxy (sidecar) containers injected into Pods.

Both inputs are technically backed by the OpenTelemetry (OTel) Collector’s Prometheus Receiver. There are five distinct scrape jobs, each targeting different configurations:

| Scrape Job | Targets |
| --- | --- |
| app-pods | Scrapes annotated Pods that either do not have an Istio sidecar or have the prometheus.io/scheme explicitly set to http. Scraping is done over plain HTTP. |
| app-services | Scrapes annotated Services backed by Pods without an Istio sidecar or where the prometheus.io/scheme is explicitly set to http (on Services). Scraping is done over plain HTTP. |
| app-pods-secure | Scrapes annotated Pods that either have an Istio sidecar or have the prometheus.io/scheme set to https. Scraping is performed over HTTPS, using a client TLS certificate injected by Istio. |
| app-services-secure | Scrapes annotated Services backed by Pods with an Istio sidecar or where the prometheus.io/scheme is set to https (on Services). Scraping is done over HTTPS, using a client TLS certificate injected by Istio. |
| istio-proxy | Scrapes metrics from Istio proxy sidecars, with scraping performed over plain HTTP. |

As discussed in this issue, the prometheusMerge feature is crucial for simplifying Dynatrace integration and enhancing security. For the sake of explanation, let’s assume that prometheusMerge is always enabled in the Istio mesh configuration. Under this scenario, the following points are true:

* Istio sidecars will merge Istio’s metrics with the application’s metrics. The combined metrics will be exposed at :15020/stats/prometheus for scraping using plain HTTP.
* The necessary prometheus.io annotations will be added to all data plane Pods to configure scraping. If these annotations already exist, they will be overwritten. It will make not possible to ditinguish between Pods that expose application metrics and those that don't.
* It is not feasible to reliably separate the merged metrics into distinct Istio and application metrics, aside from using naming conventions, which is not reliable. As a result, maintaining two separate inputs, prometheus and istio, will not represent the reality.

## Decision

