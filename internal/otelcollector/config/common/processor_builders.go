package common

import (
	"fmt"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

// =============================================================================
// KUBERNETES ATTRIBUTES PROCESSOR BUILDERS
// =============================================================================

func K8sAttributesProcessor(enrichments *operatorv1beta1.EnrichmentSpec, useOTelServiceEnrichment bool) *K8sAttributesProcessorConfig {
	k8sAttributes := []string{
		"k8s.pod.name",
		"k8s.node.name",
		"k8s.namespace.name",
		"k8s.deployment.name",
		"k8s.statefulset.name",
		"k8s.daemonset.name",
		"k8s.cronjob.name",
		"k8s.job.name",
	}

	// TODO(TeodorSAP): Move this to the slice above when old service enrichment strategy is fully deprecated
	if useOTelServiceEnrichment {
		k8sAttributes = append(k8sAttributes, "service.namespace", "service.name", "service.version", "service.instance.id")
	}

	podAssociations := []PodAssociations{
		{
			Sources: []PodAssociation{{From: "resource_attribute", Name: "k8s.pod.ip"}},
		},
		{
			Sources: []PodAssociation{{From: "resource_attribute", Name: "k8s.pod.uid"}},
		},
		{
			Sources: []PodAssociation{{From: "connection"}},
		},
	}

	return &K8sAttributesProcessorConfig{
		AuthType:    "serviceAccount",
		Passthrough: false,
		Extract: ExtractK8sMetadata{
			Metadata:                     k8sAttributes,
			Labels:                       append(extractLabels(useOTelServiceEnrichment), extractPodLabels(enrichments)...),
			Annotations:                  extractOtelServiceAnnotations(useOTelServiceEnrichment),
			OTelAnnotations:              useOTelServiceEnrichment,
			DeploymentNameFromReplicaset: useOTelServiceEnrichment,
		},
		PodAssociation: podAssociations,
	}
}

func extractLabels(useOTelServiceEnrichment bool) []ExtractLabel {
	extractLabels := []ExtractLabel{
		{
			From:    "node",
			Key:     "topology.kubernetes.io/region",
			TagName: "cloud.region",
		},
		{
			From:    "node",
			Key:     "topology.kubernetes.io/zone",
			TagName: "cloud.availability_zone",
		},
		{
			From:    "node",
			Key:     "node.kubernetes.io/instance-type",
			TagName: "host.type",
		},
		{
			From:    "node",
			Key:     "kubernetes.io/arch",
			TagName: "host.arch",
		},
	}

	// old enrichment strategy (will be deprecated)
	if !useOTelServiceEnrichment {
		extractLabels = append(
			[]ExtractLabel{
				{
					From:    "pod",
					Key:     "app.kubernetes.io/name",
					TagName: kymaK8sIOAppName,
				},
				{
					From:    "pod",
					Key:     "app",
					TagName: kymaAppName,
				},
			}, extractLabels...)
	}

	return extractLabels
}

// extractOtelServiceAnnotations returns annotation extraction config for service attributes in OTel mode.
// The extracted values are stored in temporary kyma.* attributes so a subsequent restore step can
// re-apply them after the k8sattributes processor's (buggy) label-over-annotation resolution.
func extractOtelServiceAnnotations(useOTelServiceEnrichment bool) []ExtractLabel {
	if !useOTelServiceEnrichment {
		return nil
	}

	return []ExtractLabel{
		{From: "pod", Key: otelAnnotationKeyServiceName, TagName: kymaOtelAnnotationServiceName},
		{From: "pod", Key: otelAnnotationKeyServiceVersion, TagName: kymaOtelAnnotationServiceVersion},
	}
}

func extractPodLabels(enrichments *operatorv1beta1.EnrichmentSpec) []ExtractLabel {
	extractPodLabels := make([]ExtractLabel, 0)

	if enrichments != nil && len(enrichments.ExtractPodLabels) > 0 {
		for _, label := range enrichments.ExtractPodLabels {
			labelConfig := ExtractLabel{
				From:    "pod",
				TagName: "k8s.pod.label.$0",
			}

			if label.KeyPrefix != "" {
				labelConfig.KeyRegex = fmt.Sprintf("(%s.*)", label.KeyPrefix)
			} else {
				labelConfig.KeyRegex = fmt.Sprintf("(^%s$)", label.Key)
			}

			extractPodLabels = append(extractPodLabels, labelConfig)
		}
	}

	return extractPodLabels
}

// =============================================================================
// RESOURCE PROCESSOR BUILDERS
// =============================================================================

// ResolveServiceName creates a service enrichment processor configuration
func ResolveServiceName() *ServiceEnrichmentProcessorConfig {
	return &ServiceEnrichmentProcessorConfig{
		ResourceAttributes: []string{
			kymaK8sIOAppName,
			kymaAppName,
		},
	}
}

// =============================================================================
// FILTER PROCESSOR BUILDERS
// =============================================================================

// LogFilterProcessor creates a FilterProcessorConfig for logs with error_mode set to "ignore"
func LogFilterProcessor(filters []telemetryv1beta1.FilterSpec) *FilterProcessorConfig {
	return &FilterProcessorConfig{
		ErrorMode: defaultFilterProcessorErrorMode,
		Logs:      filters,
	}
}

// MetricFilterProcessor creates a FilterProcessorConfig for metrics with the default error mode
func MetricFilterProcessor(filters []telemetryv1beta1.FilterSpec) *FilterProcessorConfig {
	return &FilterProcessorConfig{
		ErrorMode: defaultFilterProcessorErrorMode,
		Metrics:   filters,
	}
}

// TraceFilterProcessor creates a FilterProcessorConfig for traces with the default error mode
func TraceFilterProcessor(filters []telemetryv1beta1.FilterSpec) *FilterProcessorConfig {
	return &FilterProcessorConfig{
		ErrorMode: defaultFilterProcessorErrorMode,
		Traces:    filters,
	}
}

// =============================================================================
// TRANSFORM PROCESSOR BUILDERS
// =============================================================================

// LogTransformProcessor creates a TransformProcessorConfig for logs with error_mode set to "ignore"
func LogTransformProcessor(statements []TransformProcessorStatements) *TransformProcessorConfig {
	return &TransformProcessorConfig{
		ErrorMode:     defaultTransformProcessorErrorMode,
		LogStatements: statements,
	}
}

// MetricTransformProcessor creates a TransformProcessorConfig for metrics with the default error mode
func MetricTransformProcessor(statements []TransformProcessorStatements) *TransformProcessorConfig {
	return &TransformProcessorConfig{
		ErrorMode:        defaultTransformProcessorErrorMode,
		MetricStatements: statements,
	}
}

// TraceTransformProcessor creates a TransformProcessorConfig for traces with the default error mode
func TraceTransformProcessor(statements []TransformProcessorStatements) *TransformProcessorConfig {
	return &TransformProcessorConfig{
		ErrorMode:       defaultTransformProcessorErrorMode,
		TraceStatements: statements,
	}
}

// TransformSpecsToProcessorStatements converts transform specs to processor statements
func TransformSpecsToProcessorStatements(specs []telemetryv1beta1.TransformSpec) []TransformProcessorStatements {
	result := make([]TransformProcessorStatements, 0, len(specs))
	for _, spec := range specs {
		result = append(result, TransformProcessorStatements{
			Statements: spec.Statements,
			Conditions: spec.Conditions,
		})
	}

	return result
}

type ClusterOptions struct {
	ClusterName   string
	ClusterUID    string
	CloudProvider string
}

// InsertClusterAttributesProcessorStatements creates processor statements for the transform processor that inserts cluster attributes
func InsertClusterAttributesProcessorStatements(cluster ClusterOptions) []TransformProcessorStatements {
	statements := []string{
		setIfNilOrEmptyStatement("k8s.cluster.name", cluster.ClusterName),
		setIfNilOrEmptyStatement("k8s.cluster.uid", cluster.ClusterUID),
	}

	if cluster.CloudProvider != "" {
		statements = append(statements, setIfNilOrEmptyStatement("cloud.provider", cluster.CloudProvider))
	}

	return []TransformProcessorStatements{{
		Statements: statements,
	}}
}

// DropKymaAttributesProcessorStatements creates processor statements for the transform processor that drops Kyma attributes
func DropKymaAttributesProcessorStatements() []TransformProcessorStatements {
	return []TransformProcessorStatements{{
		Statements: []string{
			"delete_matching_keys(resource.attributes, \"kyma.*\")",
		},
	}}
}

// DropUnknownServiceNameProcessorStatements creates processor statements for the transform processor that drops unknown service names
func DropUnknownServiceNameProcessorStatements() []TransformProcessorStatements {
	return []TransformProcessorStatements{{
		Statements: []string{
			JoinWithWhere(
				DeleteResourceAttribute("service.name"),
				JoinWithAnd(ResourceAttributeIsNotNil("service.name"), ResourceAttributeHasPrefix("service.name", "unknown_service")),
			),
		},
	}}
}

// RestoreOtelServiceAnnotationsProcessorStatements creates processor statements that re-apply OTel pod
// annotation values for service.name and service.version after the k8sattributes processor runs.
// This is a workaround for a bug in the k8sattributes processor where pod labels (app.kubernetes.io/*)
// incorrectly take priority over pod annotations (resource.opentelemetry.io/*).
// The annotations were extracted to temporary kyma.otel.annotation.* attributes by the k8sattributes
// processor; this step restores them as the final service attribute values and deletes the temp attrs.
// Empty annotation values are treated as "not set" — the restore only fires for non-empty values so
// that the k8sattributes fallback chain (label → pod name) can produce a better result.
func RestoreOtelServiceAnnotationsProcessorStatements() []TransformProcessorStatements {
	restoreAndClean := func(serviceAttr, annotationAttr string) []string {
		return []string{
			JoinWithWhere(
				fmt.Sprintf("set(%s, %s)", ResourceAttribute(serviceAttr), ResourceAttribute(annotationAttr)),
				JoinWithAnd(ResourceAttributeIsNotNil(annotationAttr), ResourceAttributeNotEquals(annotationAttr, "")),
			),
			DeleteResourceAttribute(annotationAttr),
		}
	}

	stmts := restoreAndClean("service.name", kymaOtelAnnotationServiceName)
	stmts = append(stmts, restoreAndClean("service.version", kymaOtelAnnotationServiceVersion)...)

	return []TransformProcessorStatements{{Statements: stmts}}
}

// InstrumentationScopeProcessor creates a transform processor for instrumentation scope
func InstrumentationScopeProcessor(instrumentationScopeVersion string, inputSource ...InputSourceType) *TransformProcessorConfig {
	statements := []string{}
	transformProcessorStatements := []TransformProcessorStatements{}

	for _, i := range inputSource {
		statements = append(statements, instrumentationStatement(i, instrumentationScopeVersion)...)
	}

	transformProcessorStatements = append(transformProcessorStatements, TransformProcessorStatements{
		Statements: statements,
	})

	return MetricTransformProcessor(transformProcessorStatements)
}

// KymaInputNameProcessorStatements creates processor statements for the transform processor that sets the custom `kyma.input.name` attribute
// the attribute is mainly used for routing purpose in the metric agent configuration
func KymaInputNameProcessorStatements(inputSource InputSourceType) []TransformProcessorStatements {
	return []TransformProcessorStatements{{
		Statements: []string{
			fmt.Sprintf("set(resource.attributes[\"%s\"], \"%s\")", KymaInputNameAttribute, string(inputSource)),
		},
	}}
}

func instrumentationStatement(inputSource InputSourceType, instrumentationScopeVersion string) []string {
	return []string{
		fmt.Sprintf("set(scope.version, \"%s\") where scope.name == \"%s\"", instrumentationScopeVersion, upstreamInstrumentationScopeName[inputSource]),
		fmt.Sprintf("set(scope.name, \"%s\") where scope.name == \"%s\"", InstrumentationScope[inputSource], upstreamInstrumentationScopeName[inputSource]),
	}
}

func setIfNilOrEmptyStatement(attributeKey, attributeValue string) string {
	return JoinWithWhere(
		fmt.Sprintf("set(resource.attributes[\"%s\"], \"%s\")", attributeKey, attributeValue),
		ResourceAttributeIsNilOrEmpty(attributeKey),
	)
}
