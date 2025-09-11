package common

import (
	"fmt"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

// =============================================================================
// KUBERNETES ATTRIBUTES PROCESSOR BUILDERS
// =============================================================================

func K8sAttributesProcessorConfig(enrichments *operatorv1alpha1.EnrichmentSpec) *K8sAttributesProcessor {
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

func extractPodLabels(enrichments *operatorv1alpha1.EnrichmentSpec) []ExtractLabel {
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

// InsertClusterAttributesProcessorConfig creates a resource processor that inserts cluster attributes
func InsertClusterAttributesProcessorConfig(clusterName, clusterUID, cloudProvider string) *ResourceProcessor {
	if cloudProvider != "" {
		return &ResourceProcessor{
			Attributes: []AttributeAction{
				{
					Action: "insert",
					Key:    "k8s.cluster.name",
					Value:  clusterName,
				},
				{
					Action: "insert",
					Key:    "k8s.cluster.uid",
					Value:  clusterUID,
				},
				{
					Action: "insert",
					Key:    "cloud.provider",
					Value:  cloudProvider,
				},
			},
		}
	}

	return &ResourceProcessor{
		Attributes: []AttributeAction{
			{
				Action: "insert",
				Key:    "k8s.cluster.name",
				Value:  clusterName,
			},
			{
				Action: "insert",
				Key:    "k8s.cluster.uid",
				Value:  clusterUID,
			},
		},
	}
}

// DropKymaAttributesProcessorConfig creates a resource processor that drops Kyma attributes
func DropKymaAttributesProcessorConfig() *ResourceProcessor {
	return &ResourceProcessor{
		Attributes: []AttributeAction{
			{
				Action:       "delete",
				RegexPattern: "kyma.*",
			},
		},
	}
}

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
func TransformSpecsToProcessorStatements(specs []telemetryv1alpha1.TransformSpec) []TransformProcessorStatements {
	result := make([]TransformProcessorStatements, 0, len(specs))
	for _, spec := range specs {
		result = append(result, TransformProcessorStatements{
			Statements: spec.Statements,
			Conditions: spec.Conditions,
		})
	}

	return result
}

// InstrumentationScopeProcessorConfig creates a transform processor for instrumentation scope
func InstrumentationScopeProcessorConfig(instrumentationScopeVersion string, inputSource ...InputSourceType) *TransformProcessor {
	statements := []string{}
	transformProcessorStatements := []TransformProcessorStatements{}

	for _, i := range inputSource {
		statements = append(statements, instrumentationStatement(i, instrumentationScopeVersion)...)

		if i == InputSourcePrometheus {
			transformProcessorStatements = append(transformProcessorStatements, TransformProcessorStatements{
				Statements: []string{fmt.Sprintf("set(resource.attributes[\"%s\"], \"%s\")", KymaInputNameAttribute, KymaInputPrometheus)},
			})
		}
	}

	transformProcessorStatements = append(transformProcessorStatements, TransformProcessorStatements{
		Statements: statements,
	})

	return MetricTransformProcessorConfig(transformProcessorStatements)
}

func instrumentationStatement(inputSource InputSourceType, instrumentationScopeVersion string) []string {
	return []string{
		fmt.Sprintf("set(scope.version, \"%s\") where scope.name == \"%s\"", instrumentationScopeVersion, upstreamInstrumentationScopeName[inputSource]),
		fmt.Sprintf("set(scope.name, \"%s\") where scope.name == \"%s\"", InstrumentationScope[inputSource], upstreamInstrumentationScopeName[inputSource]),
	}
}
