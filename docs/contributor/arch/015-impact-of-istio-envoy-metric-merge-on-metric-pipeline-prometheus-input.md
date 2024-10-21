# 11. Impact of Istio-Envoy Metrics Merge Feature on MetricPipeline's Prometheus Input

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

## Decision

