# 11. Impact of Istio-Envoy Metrics Merge Feature on MetricPipeline's Prometheus Input

Date: 2024-10-21

## Status

Proposed

## Context

MetricPipeline amongst other input types supports `prometheus` and `istio` inputs . If `prometheus` input is enabled, Pods and Services marked with `prometheus.io/scrape=true` annotation are scraped. If `istio` input is enabled, istio-proxy container metrics are scraped from Pods that have had the istio-proxy sidecar injected.

Technically both inputs are backed by OTel Collector Prometheus Receiver. There are 5 scrape jobs:
| Scrape Job | Description |
| --- | --- |
| app-pods | Annotated Pods without Istio Proxy sidecar or `prometheus.io/scheme` explicitly set to `http`. Scraping is performed using plain http. |
| app-services |  Annotated Services backed by Pods without sidecar or `prometheus.io/scheme` explicitly set to `http` (on Services). Scraping is performed using http. |
| app-pods-secure |  Annotated Pods with sidecar or `prometheus.io/scheme` explicitly set to `https`. Scraping is performed over https using a client TLC certificate injected by istio. |
| app-services-secure |  Annotated Services backed by Pods with sidecar or `prometheus.io/scheme` explicitly set to `https` (on Services). Scraping is performed over https using a client TLC certificate injected by istio. |
| istio |
## Decision

