---
title: Service Attributes Enrichment using Consistent OTel Approach
status: Proposed
date: 2026-01-12
---

# 30: Service Attributes Enrichment using Consistent OTel Approach

## Context

The otel-collector recently introduced proper service attributes (`service.namespace`, `service.name`, `service.version`, `service.instance.id`) enrichment as part of the `k8sattributes` processor (https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/39335/files).

The conventions and enrichment fallback chains that are implemented are documented here: https://opentelemetry.io/docs/specs/semconv/non-normative/k8s-attributes/#service-attributes

Currently, we are performing the enrichment in a custom way by using our custom `servicenameenrichment` processor. Therefore, we should aim to use the upstream feature and eliminate the custom logic.

A first challenge here is that the Istio trace spans are enriched with the service name attribute by Istio itself, following custom Istio logic. In our case, this logic is configured in the [MeshConfig of the Kyma Istio module](https://github.com/kyma-project/istio/blob/6295e154b3992cf42c44d40eed3c2ec488f990bf/internal/istiooperator/istio-operator.yaml#L237), in which the `TracingServiceName` field is set to `CANONICAL_NAME_ONLY` (https://istio.io/latest/docs/reference/config/istio.mesh.v1alpha1/#ProxyConfig-TracingServiceName). This, in turn, enriches the `service.name` attribute of Istio-generated trace spans with the canonical name for a workload, which results in the following fallback chain enrichment logic (in order):
- `service.istio.io/canonical-name` label (https://github.com/istio/istio/blob/master/pkg/model/proxy.go#L492)
- `app.kubernetes.io/name` label
- `app` label
- `"istio-proxy"`

A second challenge is represented by the fact that this change will be breaking for our current users, as currently enriched telemetry data does not follow the same OTel convention (fallback logic) for setting the service attributes.

## Proposal

### First Challenge: Overwriting Istio Trace Spans
As we actively enrich Istio data, we should overwrite the Istio enrichment (as a documented feature) to make that enrichment consistent across everything.

In order to correctly identify Istio-generated trace spans, there are mainly two options:
1. Adding a custom attribute to the span (from Istio's side)
2. Using Istio-specific attributes that are already set on the span

#### Option 1: Setting A Custom Attribute

Although this approach would be more consistent with our current way of handling Istio access logs, as of today there are no configuration options on Istio's side that would allow setting a custom attribute for OTel tracing.

There is the option of enabling the [environment resource detector in Istio's mesh config](https://istio.io/latest/docs/tasks/observability/distributed-tracing/opentelemetry/#exporting-via-grpc), and thus setting custom attributes based on the `OTEL_RESOURCE_ATTRIBUTES` environment variable, but this requires access to the user's application deployment, which is not feasible in this case.

#### Option 2: Using Istio-Specific Attributes

This approach uses Istio's already set attributes in order to identify its spans. An example of an Istio-generated trace span's attributes is the following:
```yaml
Attributes:
      -> node_id: STRING(sidecar~10.244.0.8~productpage-v1-564d4686f-t6s4m.default~default.svc.cluster.local)
      -> zone: STRING()
      -> guid:x-request-id: STRING(da543297-0dd6-998b-bd29-fdb184134c8c)
      -> http.url: STRING(http://reviews:9080/reviews/0)
      -> http.method: STRING(GET)
      -> downstream_cluster: STRING(-)
      -> user_agent: STRING(curl/7.74.0)
      -> http.protocol: STRING(HTTP/1.1)
      -> peer.address: STRING(10.244.0.8)
      -> request_size: STRING(0)
      -> response_size: STRING(441)
      -> component: STRING(proxy)
      -> upstream_cluster: STRING(outbound|9080||reviews.default.svc.cluster.local)
      -> upstream_cluster.name: STRING(outbound|9080||reviews.default.svc.cluster.local)
      -> http.status_code: STRING(200)
      -> response_flags: STRING(-)
      -> istio.namespace: STRING(default)
      -> istio.canonical_service: STRING(productpage)
      -> istio.mesh_id: STRING(cluster.local)
      -> istio.canonical_revision: STRING(v1)
      -> istio.cluster_id: STRING(Kubernetes)
```

We are already using the `component` attribute in order to identify Istio trace spans in the `istionoisefilter` processor:
```go
// component must be "proxy" to be considered an Istio proxy span.
isIstioProxy := attrs.component == "proxy"
```
> Source: https://github.com/TeodorSAP/opentelemetry-collector-components/blob/test/empty-service-name/processor/istionoisefilter/internal/rules/span.go#L12

Once identified, these trace spans can be easily dropped, by using an OTel transform processor before the `k8sattributes` processor, which will then enrich them properly. See example below:
```yaml
# ...
processors:
  transform/drop-istio-service-name:
    trace_statements:
      - delete_key(resource.attributes, "service.name") where span.attributes["component"] == "proxy"
```

### Second Challenge: Incrementally Introducing This Breaking Change
TODO

## Decision