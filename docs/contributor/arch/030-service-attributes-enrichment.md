---
title: Service Attributes Enrichment using Consistent OTel Approach
status: Accepted
date: 2026-01-12
---

# 30: Service Attributes Enrichment using Consistent OTel Approach

## Context

With [PR 39335](https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/39335/files), the OTel Collector `k8sattributes` processor introduced enrichment with service attributes (`service.namespace`, `service.name`, `service.version`, `service.instance.id`).

For the conventions and enrichment fallback chains, see [OpenTelemetry: Service attributes](https://opentelemetry.io/docs/specs/semconv/non-normative/k8s-attributes/#service-attributes).

Currently, we use our custom `servicenameenrichment` processor for this enrichment. We should use the standard `k8sattributes` processor to act according to the official OTel conventions and eliminate our custom logic in the process.

A first challenge is that Istio enriches trace spans with the **service.name** attribute using its own custom logic. In our case, this logic is configured in the [MeshConfig of the Kyma Istio module](https://github.com/kyma-project/istio/blob/6295e154b3992cf42c44d40eed3c2ec488f990bf/internal/istiooperator/istio-operator.yaml#L237), setting the `TracingServiceName` field to `CANONICAL_NAME_ONLY` (https://istio.io/latest/docs/reference/config/istio.mesh.v1alpha1/#ProxyConfig-TracingServiceName). This, in turn, enriches the `service.name` attribute of Istio-generated trace spans with the canonical name for a workload, which uses the following fallback chain for enrichment:
1. `service.istio.io/canonical-name` label (https://github.com/istio/istio/blob/master/pkg/model/proxy.go#L492)
2. `app.kubernetes.io/name` label
3. `app` label
4. `"istio-proxy"`

As a second challenge, this is a breaking change for our users, because the new OTel convention for setting service attributes differs from our current implementation.

## Proposal

### First Challenge: Overwriting Istio Trace Spans
Ideally, Istio would follow the OTel conventions natively or provide configuration options to do so. Since this is not the case, we need to compensate by actively modifying the data. Our OTel Collector configuration is the best place for this compensation, as it already processes Istio spans and access logs. Thus, we should modify the default Istio service attributes enrichment to ensure uniform enrichment logic across all telemetry data, according to OTel conventions.

To identify Istio-generated trace spans, we have the following options:
1. Adding a custom attribute to the span (from Istio's side)
2. Using Istio-specific attributes that are already set on the span

> [!NOTE]
> An issue was raised in the Istio repository to add OTel-compliant service attribute enrichment: https://github.com/istio/istio/issues/58803. This was resolved via [istio/api#3653](https://github.com/istio/api/pull/3653) and [istio/istio#59207](https://github.com/istio/istio/pull/59207), introducing the `serviceAttributeEnrichment: OTEL_SEMANTIC_CONVENTIONS` field under `meshConfig.extensionProviders[].opentelemetry`. Once the Kyma Istio module adopts this change, the `transform/drop-istio-service-name` OTTL transform processor workaround becomes obsolete and can be removed.

#### Option 1: Setting A Custom Attribute

Although this approach is more consistent with our current way of handling Istio access logs, Istio currently offers no configuration options to set a custom attribute for OTel tracing.

Istio provides a way to set custom attributes with the `OTEL_RESOURCE_ATTRIBUTES` environment variable (see [Istio: Exporting via gRPC](https://istio.io/latest/docs/tasks/observability/distributed-tracing/opentelemetry/#exporting-via-grpc)), but this requires modifying the user's application deployment. This approach is not feasible because it would require manual configuration from the user for each application.

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

Once identified, we can use an OTel transform processor to drop the **service.name** attribute from these spans. This processor runs before the `k8sattributes` processor, which then correctly enriches the spans. See example below:
```yaml
# ...
processors:
  transform/drop-istio-service-name:
    trace_statements:
      - delete_key(resource.attributes, "service.name") where span.attributes["component"] == "proxy"
```

### Second Challenge: Incrementally Introducing This Breaking Change

To minimize disruption to existing users, we roll out the migration from the custom `servicenameenrichment` processor to the standard OTel `k8sattributes` processor in four phases. This approach ensures backward compatibility and gives users time to adapt to the new behavior.

#### Annotation-Based Processor Selection

On the Telemetry CR, we introduce a custom annotation `telemetry.kyma-project.io/service-enrichment` to control which service enrichment processor is applied across all telemetry types and pipelines (traces, metrics, and logs). This annotation accepts the following values:

- **Unset**: For existing Telemetry resources, the annotation is unset. New Telemetry CRs will have this annotation automatically set to `otel`. When unset, the default processor behavior depends on the migration phase:
  - **Phase 1**: Uses `servicenameenrichment` processor (legacy behavior)
  - **Phase 2**: Uses `servicenameenrichment` processor (legacy behavior); new default Telemetry CRs have the annotation set to `otel`
  - **Phase 3**: Uses `k8sattributes` processor (new behavior)
  - **Phase 4**: Annotation support removed; always uses `k8sattributes` processor
- **`otel`**: Explicitly use the standard OTel `k8sattributes` processor
- **`kyma-legacy`**: Explicitly use our legacy custom `servicenameenrichment` processor

This mechanism manages the transition at the cluster level, uniformly affecting all telemetry pipelines and preserving backward compatibility for existing deployments.

#### Phase 1: Introduce New Enrichment Logic

In this initial phase, we introduce the new enrichment logic and the annotation-based selection mechanism. We implement the `transform/drop-istio-service-name` OTTL transform processor, configure `k8sattributes` to enrich service attributes, and introduce the `telemetry.kyma-project.io/service-enrichment` annotation on the Telemetry CR.

**Deliverables:**
- Implement the custom transform processor that drops Istio trace spans' `service.name` enrichment attribute (identified by `component="proxy"`)
- Configure the `k8sattributes` processor to enrich service attributes
- Implement the `telemetry.kyma-project.io/service-enrichment` annotation for switching between the legacy `servicenameenrichment` processor and the new logic

**Default Behavior:**
- **New Telemetry resources**: Annotation set to `otel` (uses new processor).
- **Existing Telemetry resources (annotation unset)**: Use `servicenameenrichment` processor (preserves existing behavior).

> [!NOTE]
> The Kyma Lifecycle Manager (KLM) applies the default Telemetry CR only once during module enablement. Subsequent module upgrades do not overwrite existing Telemetry resources, preserving custom configurations and annotations. As an edge case, if users manually delete the Telemetry CR, KLM reapplies the default configuration on the next reconciliation. This behavior will be addressed in future improvements.

**User Action (Optional):**
To adopt the new processor proactively, users can set the annotation manually, which also removes any warning condition in the Telemetry CR status:

```yaml
apiVersion: telemetry.kyma-project.io/v1beat1
kind: Telemetry
metadata:
  name: sample
  annotations:
    # Recommended: Adopt the new processor early
    telemetry.kyma-project.io/service-enrichment: otel
    
    # Alternative: Explicitly maintain legacy behavior (not recommended)
    # telemetry.kyma-project.io/service-enrichment: kyma-legacy
    
    # Default: Leave unset to continue using servicenameenrichment processor
spec:
  # ...
```

#### Phase 2: New Clusters Use New Logic

In this phase, the default Telemetry CR is updated so that newly created resources have the annotation set to `otel` by default. Existing resources with the annotation unset continue using the legacy `servicenameenrichment` processor.

**Deliverables:**
- Configure the default Telemetry CR so that newly created resources have the annotation set to `otel`
- Add a warning banner to Kyma Dashboard when the annotation is not present with the new value, warning users about the future deprecation
- Document the deprecation process
- Implement a custom metric that reflects the feature adoption rate
- Update dashboards to track the new metric

#### Monitoring Adoption Rates

To track the adoption of the new processor during phases 2 and 3, the Telemetry Manager must export a new metric:

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
count(telemetry_service_enrichment_processor_usage{processor_type="otel"})
count(telemetry_service_enrichment_processor_usage{processor_type="kyma-legacy"})
count(telemetry_service_enrichment_processor_usage{processor_type="unset"})
```

#### Phase 3: Deprecation with Backward Compatibility

In this phase, the default behavior changes: resources with unset annotations now use the `k8sattributes` processor. However, users that need more time for migration can still explicitly choose the legacy `servicenameenrichment` processor.

**Default Behavior (Annotation Unset):**
- **All Telemetry resources**: Use `k8sattributes` processor by default.
- **Warning condition**: Telemetry CRs with the legacy processor explicitly set will show deprecation warnings as status conditions.

**Deprecation Warning:**
When the annotation is set to `kyma-legacy`, the Telemetry CR includes a warning status condition notifying users that the `servicenameenrichment` processor is deprecated and will be removed in the future.

**User Action (Optional):**
Users that need more time for migration can temporarily revert to the legacy processor with the following annotation:

```yaml
apiVersion: telemetry.kyma-project.io/v1beat1
kind: Telemetry
metadata:
  name: sample
  annotations:
    # Temporarily opt back into legacy processor
    telemetry.kyma-project.io/service-enrichment: kyma-legacy
spec:
  # ...
```

#### Phase 4: Cleanup

In the final phase, we remove the annotation-based selection mechanism and all related legacy code. This phase requires a 3-month deprecation notice upfront.

**Deliverables:**
- Change the default behavior to always use the new enrichment logic
- Remove unused configuration, code, and monitoring metric (for example, reconcilers, config builders)
- Delete `service_name` end-to-end tests and adapt `service_enrichment` tests according to the comments
- Archive and remove the `servicenameenrichment` processor from OCC
- Delete the deprecation process documentation

**Default Behavior:**
- **All Telemetry resources**: Use OTel's `k8sattributes` processor exclusively.
- **Annotation removal**: The `telemetry.kyma-project.io/service-enrichment` annotation is no longer supported.

This phase marks the completion of the migration to standards-compliant OTel attribute enrichment.


## Decision

We will migrate from the custom `servicenameenrichment` processor to the OTel-native `k8sattributes` processor to align with OpenTelemetry conventions for service attribute enrichment across all telemetry types (traces, metrics, and logs). To handle Istio-generated trace spans that are pre-enriched by Istio's MeshConfig, we will use an OTel transform processor to remove the `service.name` attribute from spans identified by `component="proxy"` before applying the `k8sattributes` processor, ensuring consistent enrichment logic. The `transform/drop-istio-service-name` OTTL transform processor workaround must be kept until the Kyma Istio module adopts the `serviceAttributeEnrichment: OTEL_SEMANTIC_CONVENTIONS` field introduced in [istio/api#3653](https://github.com/istio/api/pull/3653) and [istio/istio#59207](https://github.com/istio/istio/pull/59207). Once adopted, this first challenge becomes obsolete, meaning the OTTL transform processor workaround can be removed and istio's `meshConfig` adapted accordingly.

We execute the migration in four phases controlled by the `telemetry.kyma-project.io/service-enrichment` annotation on the Telemetry CR:
- **Phase 1** introduces the annotation, the `transform/drop-istio-service-name` processor, and the updated `k8sattributes` configuration. New Telemetry resources automatically get the annotation set to `otel`; existing resources with unset annotations default to the legacy `servicenameenrichment` processor.
- **Phase 2** updates the default Telemetry CR so newly created resources use the new logic by default, adds a Kyma Dashboard deprecation warning for clusters without the annotation set to `otel`, documents the deprecation process, and introduces the `telemetry_service_enrichment_processor_usage` operator metric with dashboard support to monitor adoption rates.
- **Phase 3** changes the default behavior so that unset annotations use the `k8sattributes` processor, while still allowing explicit fallback to the legacy processor with a deprecation warning status condition on the Telemetry CR.
- **Phase 4** removes annotation support entirely, enforces the `k8sattributes` processor universally, removes all unused code and monitoring metrics, and archives the `servicenameenrichment` processor from OCC.
