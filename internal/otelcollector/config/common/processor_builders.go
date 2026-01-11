package common

import (
	"fmt"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

// =============================================================================
// KUBERNETES ATTRIBUTES PROCESSOR BUILDERS
// =============================================================================

func K8sAttributesProcessorConfig(enrichments *operatorv1beta1.EnrichmentSpec) *K8sAttributesProcessor {
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

	return &K8sAttributesProcessor{
		AuthType:    "serviceAccount",
		Passthrough: false,
		Extract: ExtractK8sMetadata{
			Metadata: k8sAttributes,
			Labels:   append(extractLabels(), extractPodLabels(enrichments)...),
		},
		PodAssociation: podAssociations,
	}
}

func extractLabels() []ExtractLabel {
	return []ExtractLabel{
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

// ResolveServiceNameConfig creates a service enrichment processor configuration
func ResolveServiceNameConfig() *ServiceEnrichmentProcessor {
	return &ServiceEnrichmentProcessor{
		ResourceAttributes: []string{
			kymaK8sIOAppName,
			kymaAppName,
		},
	}
}

// =============================================================================
// FILTER PROCESSOR BUILDERS
// =============================================================================

// LogFilterProcessorConfig creates a FilterProcessor for logs with error_mode set to "ignore"
func LogFilterProcessorConfig(logs FilterProcessorLogs) *FilterProcessor {
	return &FilterProcessor{
		ErrorMode: defaultFilterProcessorErrorMode,
		Logs:      logs,
	}
}

// MetricFilterProcessorConfig creates a FilterProcessor for metrics with the default error mode
func MetricFilterProcessorConfig(metrics FilterProcessorMetrics) *FilterProcessor {
	return &FilterProcessor{
		ErrorMode: defaultFilterProcessorErrorMode,
		Metrics:   metrics,
	}
}

// TraceFilterProcessorConfig creates a FilterProcessor for traces with the default error mode
func TraceFilterProcessorConfig(traces FilterProcessorTraces) *FilterProcessor {
	return &FilterProcessor{
		ErrorMode: defaultFilterProcessorErrorMode,
		Traces:    traces,
	}
}

func FilterSpecsToLogFilterProcessorConfig(specs []telemetryv1beta1.FilterSpec) *FilterProcessor {
	var mergedConditions []string
	for _, spec := range specs {
		mergedConditions = append(mergedConditions, spec.Conditions...)
	}

	return &FilterProcessor{
		ErrorMode: defaultFilterProcessorErrorMode,
		Logs: FilterProcessorLogs{
			// Use log context as it is the lowest one and it is always present
			Log: mergedConditions,
		},
	}
}

func FilterSpecsToMetricFilterProcessorConfig(specs []telemetryv1beta1.FilterSpec) *FilterProcessor {
	var mergedConditions []string
	for _, spec := range specs {
		mergedConditions = append(mergedConditions, spec.Conditions...)
	}

	return &FilterProcessor{
		ErrorMode: defaultFilterProcessorErrorMode,
		Metrics: FilterProcessorMetrics{
			// Use datapoint context as it is the lowest one and it is always present
			Datapoint: mergedConditions,
		},
	}
}

func FilterSpecsToTraceFilterProcessorConfig(specs []telemetryv1beta1.FilterSpec) *FilterProcessor {
	var mergedConditions []string
	for _, spec := range specs {
		mergedConditions = append(mergedConditions, spec.Conditions...)
	}

	return &FilterProcessor{
		ErrorMode: defaultFilterProcessorErrorMode,
		Traces: FilterProcessorTraces{
			// Use span as context instead of spanevents, because while more granular, spanevents aren't always present
			// span event filtering is not supported by user-defined filter until filter processor supports context inference
			Span: mergedConditions,
		},
	}
}

// =============================================================================
// TRANSFORM PROCESSOR BUILDERS
// =============================================================================

// LogTransformProcessorConfig creates a TransformProcessor for logs with error_mode set to "ignore"
func LogTransformProcessorConfig(statements []TransformProcessorStatements) *TransformProcessor {
	return &TransformProcessor{
		ErrorMode:     defaultTransformProcessorErrorMode,
		LogStatements: statements,
	}
}

// MetricTransformProcessorConfig creates a TransformProcessor for metrics with the default error mode
func MetricTransformProcessorConfig(statements []TransformProcessorStatements) *TransformProcessor {
	return &TransformProcessor{
		ErrorMode:        defaultTransformProcessorErrorMode,
		MetricStatements: statements,
	}
}

// TraceTransformProcessorConfig creates a TransformProcessor for traces with the default error mode
func TraceTransformProcessorConfig(statements []TransformProcessorStatements) *TransformProcessor {
	return &TransformProcessor{
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

// InstrumentationScopeProcessorConfig creates a transform processor for instrumentation scope
func InstrumentationScopeProcessorConfig(instrumentationScopeVersion string, inputSource ...InputSourceType) *TransformProcessor {
	statements := []string{}
	transformProcessorStatements := []TransformProcessorStatements{}

	for _, i := range inputSource {
		statements = append(statements, instrumentationStatement(i, instrumentationScopeVersion)...)
	}

	transformProcessorStatements = append(transformProcessorStatements, TransformProcessorStatements{
		Statements: statements,
	})

	return MetricTransformProcessorConfig(transformProcessorStatements)
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
