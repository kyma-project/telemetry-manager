---
title: Service Attributes Enrichment using Consistent OTel Approach
status: Proposed
date: 2026-01-12
---

# 30: Service Attributes Enrichment using Consistent OTel Approach

## Context

The OTel Collector recently introduced proper service attributes (`service.namespace`, `service.name`, `service.version`, `service.instance.id`) enrichment as part of the `k8sattributes` processor (https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/39335/files).

For the conventions and enrichment fallback chains see [OpenTelemetry: Service attributes](https://opentelemetry.io/docs/specs/semconv/non-normative/k8s-attributes/#service-attributes).

Currently, we are performing the enrichment in a custom way by using our custom `servicenameenrichment` processor. Therefore, we should aim to use the upstream feature and eliminate the custom logic.

A first challenge is that the Istio trace spans are enriched with the service name attribute by Istio, following custom Istio logic. In our case, this logic is configured in the [MeshConfig of the Kyma Istio module](https://github.com/kyma-project/istio/blob/6295e154b3992cf42c44d40eed3c2ec488f990bf/internal/istiooperator/istio-operator.yaml#L237), setting the `TracingServiceName` field to `CANONICAL_NAME_ONLY` (https://istio.io/latest/docs/reference/config/istio.mesh.v1alpha1/#ProxyConfig-TracingServiceName). This, in turn, enriches the `service.name` attribute of Istio-generated trace spans with the canonical name for a workload, which results in the following fallback chain enrichment logic (in order):
1. `service.istio.io/canonical-name` label (https://github.com/istio/istio/blob/master/pkg/model/proxy.go#L492)
2. `app.kubernetes.io/name` label
3. `app` label
4. `"istio-proxy"`

As a second challenge, this would be a breaking change for our current users, because currently enriched telemetry data does not follow the same OTel convention (fallback logic) for setting the service attributes.

## Proposal

### First Challenge: Overwriting Istio Trace Spans
Since Telemetry Manager actively enriches Istio data, we should overwrite the default Istio service attributes enrichment (as a documented feature) to ensure a uniform enrichment logic across every telemetry data, according to the OTel conventions.

To identify Istio-generated trace spans, we have the following options:
1. Adding a custom attribute to the span (from Istio's side)
2. Using Istio-specific attributes that are already set on the span

#### Option 1: Setting A Custom Attribute

Although this approach is more consistent with our current way of handling Istio access logs, Istio currently offers no configuration options to set a custom attribute for OTel tracing.

There is the option of enabling the [environment resource detector in Istio's mesh config](https://istio.io/latest/docs/tasks/observability/distributed-tracing/opentelemetry/#exporting-via-grpc), and thus setting custom attributes based on the `OTEL_RESOURCE_ATTRIBUTES` environment variable, but this requires access to the user's application deployment, which is not feasible in this case.

#### Option 2: Using Istio-Specific Attributes

This approach uses Istio's already set attributes to identify its spans. See the following example of an Istio-generated trace span's attributes:
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

We are already using the `component` attribute to identify Istio trace spans in the `istionoisefilter` processor:
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

To minimize disruption to existing users, we roll out the migration from the custom `servicenameenrichment` processor to the standard OTel `k8sattributes` processor in three phases. This approach ensures backward compatibility and gives users time to adapt to the new behavior.

#### Annotation-Based Processor Selection

On the Telemetry CR, we introduce a custom annotation `kyma-project.io/telemetry-service-enrichment` to control which service enrichment processor is applied across all telemetry types and pipelines (traces, metrics, and logs). This annotation accepts the following values:

- **Unset**: For existing Telemetry resources, the annotation is unset. New Telemetry CRs will have this annotation automatically set to `k8sattributes`. When unset, the default processor behavior depends on the migration phase:
  - **Phase 1**: Uses `servicenameenrichment` processor (legacy behavior)
  - **Phase 2**: Uses `k8sattributes` processor (new behavior)
  - **Phase 3**: Annotation support removed; always uses `k8sattributes` processor
- **`k8sattributes`**: Explicitly use the standard OTel `k8sattributes` processor
- **`servicenameenrichment`**: Explicitly use the legacy custom `servicenameenrichment` processor

This mechanism manages the transition at the cluster level, uniformly affecting all telemetry pipelines and preserving backward compatibility for existing deployments.

#### Phase 1: Introduction with Opt-In (Suggested Feature)

In this initial phase, we introduce the annotation, automatically set to `k8sattributes` for new Telemetry resources. Existing Telemetry resources have the annotation unset, defaulting to the legacy `servicenameenrichment` processor to ensure no breaking changes.

**Default Behavior:**
- **New Telemetry resources**: Annotation set to `k8sattributes` (uses new processor)
- **Existing Telemetry resources (annotation unset)**: Use `servicenameenrichment` processor (preserves existing behavior)
- **Warning condition**: Added to the Telemetry CR status for existing resources, recommending migration to the new processor

> [!NOTE]
> The Kyma Lifecycle Manager (KLM) applies the default Telemetry CR only once during module enablement. Subsequent module upgrades do not overwrite existing Telemetry resources, preserving custom configurations and annotations. As an edge case, if users manually delete the Telemetry CR, KLM reapplies the default configuration on the next reconciliation. This behavior will be addressed in future improvements.

**Status Condition:**
The Telemetry CR will include a warning condition similar to `CertAboutToExpire` indicating the deprecation status and recommending adoption of the new processor.

**User Action (Optional):**
If users want to adopt the new processor proactively, they can set the annotation manually, which also removes the warning condition in the Telemetry CR status:

```yaml
apiVersion: telemetry.kyma-project.io/v1beat1
kind: Telemetry
metadata:
  name: sample
  annotations:
    # Recommended: Adopt the new processor early
    kyma-project.io/telemetry-service-enrichment: k8sattributes
    
    # Alternative: Explicitly maintain legacy behavior (not recommended)
    # kyma-project.io/telemetry-service-enrichment: servicenameenrichment
    
    # Default: Leave unset to continue using servicenameenrichment processor
spec:
  # ...
```

#### Phase 2: Deprecation with Backward Compatibility

In this phase, the default behavior changes: Resources with unset annotations now use the `k8sattributes` processor. However, users that need more time for migration can still explicitly choose the legacy `servicenameenrichment` processor.

**Default Behavior (Annotation Unset):**
- **All Telemetry resources**: Use `k8sattributes` processor by default
- **Enhanced warnings**: Telemetry CRs with the legacy processor explicitly set will show stronger deprecation warnings

**Status Condition:**
If the annotation is set to `servicenameenrichment`, then the Telemetry CR includes a warning condition notifying users that the `servicenameenrichment` processor is deprecated and will be removed in the future.

**User Action (Optional):**
Users that need more time for migration can temporarily revert to the legacy processor with the following annotation:

```yaml
apiVersion: telemetry.kyma-project.io/v1beat1
kind: Telemetry
metadata:
  name: sample
  annotations:
    # Temporarily opt back into legacy processor
    kyma-project.io/telemetry-service-enrichment: servicenameenrichment
spec:
  # ...
```

#### Phase 3: Complete Migration

In the final phase, we remove the annotation-based selection mechanism, and all resources use the standard OTel `k8sattributes` processor.

**Default Behavior:**
- **All Telemetry resources**: Use OTel's `k8sattributes` processor exclusively
- **Annotation removal**: The `kyma-project.io/telemetry-service-enrichment` annotation is no longer supported

This phase marks the completion of the migration to standards-compliant OTel attribute enrichment.

#### Monitoring Adoption Rates

To track the adoption of the new processor during phase 1 and phase 2, the Telemetry Manager must export a new metric:

```go
ServiceEnrichmentProcessorUsage = promauto.With(registry).NewGaugeVec(
    prometheus.GaugeOpts{
        Namespace: "telemetry",
        Name:      "service_enrichment_processor_usage",
        Help:      "Service enrichment processor type in use by Telemetry resources",
    },
    []string{"processor_type"},
)
```

With this metric, we can monitor adoption progress and identify clusters that need support during the transition. 

We can use dashboards to visualize the data, and run queries such as the following:

```promql
# Count Telemetry resources using a specific processor type
count(telemetry_service_enrichment_processor_usage{processor_type="k8sattributes"})
count(telemetry_service_enrichment_processor_usage{processor_type="servicenameenrichment"})
count(telemetry_service_enrichment_processor_usage{processor_type="unset"})
```


## Decision

We will migrate from the custom `servicenameenrichment` processor to the OTel-native `k8sattributes` processor to align with OpenTelemetry conventions for service attribute enrichment across all telemetry types (traces, metrics, and logs). To handle Istio-generated trace spans that are pre-enriched by Istio's MeshConfig, we will use an OTel transform processor to remove the `service.name` attribute from spans identified by `component="proxy"` before applying the `k8sattributes` processor, ensuring consistent enrichment logic. We execute the migration in three phases controlled by the `kyma-project.io/telemetry-service-enrichment` annotation on the Telemetry CR:
- **Phase 1** introduces the annotation, automatically setting it to `k8sattributes` for new resources while leaving existing resources unset (defaulting to legacy `servicenameenrichment` processor) with warning conditions encouraging migration.
- **Phase 2** changes the default behavior so unset annotations use `k8sattributes`, while still allowing explicit fallback to the legacy processor.
- **Phase 3** removes annotation support entirely, enforcing the `k8sattributes` processor universally. Throughout the migration, we use operator-exported metrics (`telemetry_service_enrichment_processor_usage`) to monitor adoption rates across clusters and provide targeted support where needed.
