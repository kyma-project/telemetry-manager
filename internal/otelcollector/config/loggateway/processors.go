package loggateway

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
)

func processorsConfig(opts BuildOptions) Processors {
	return Processors{
		BaseProcessors: common.BaseProcessors{
			Batch:         batchProcessorConfig(),
			MemoryLimiter: memoryLimiterProcessorConfig(),
		},
		SetObsTimeIfZero:        setObsTimeIfZeroProcessorConfig(),
		IstioNoiseFilter:        &common.IstioNoiseFilterProcessor{},
		K8sAttributes:           common.K8sAttributesProcessorConfig(opts.Enrichments),
		InsertClusterAttributes: common.InsertClusterAttributesProcessorConfig(opts.ClusterName, opts.ClusterUID, opts.CloudProvider),
		ResolveServiceName:      common.ResolveServiceNameConfig(),
		DropKymaAttributes:      common.DropKymaAttributesProcessorConfig(),
		IstioEnrichment:         istioEnrichmentProcessorConfig(opts),
		Dynamic:                 make(map[string]any),
	}
}

func istioEnrichmentProcessorConfig(opts BuildOptions) *IstioEnrichmentProcessor {
	return &IstioEnrichmentProcessor{
		ScopeVersion: opts.ModuleVersion,
	}
}

//nolint:mnd // hardcoded values
func batchProcessorConfig() *common.BatchProcessor {
	return &common.BatchProcessor{
		SendBatchSize:    512,
		Timeout:          "10s",
		SendBatchMaxSize: 512,
	}
}

//nolint:mnd // hardcoded values
func memoryLimiterProcessorConfig() *common.MemoryLimiter {
	return &common.MemoryLimiter{
		CheckInterval:        "1s",
		LimitPercentage:      75,
		SpikeLimitPercentage: 15,
	}
}

func setObsTimeIfZeroProcessorConfig() *common.TransformProcessor {
	return common.LogTransformProcessor([]common.TransformProcessorStatements{{
		Conditions: []string{"log.observed_time_unix_nano == 0"},
		Statements: []string{"set(log.observed_time, Now())"},
	}})
}

func namespaceFilterProcessorConfig(namespaceSelector *telemetryv1alpha1.NamespaceSelector) *FilterProcessor {
	var filterExpressions []string

	if len(namespaceSelector.Exclude) > 0 {
		namespacesConditions := namespacesConditions(namespaceSelector.Exclude)

		// Drop logs if the excluded namespaces are matched
		excludeNamespacesExpr := common.JoinWithOr(namespacesConditions...)
		filterExpressions = append(filterExpressions, excludeNamespacesExpr)
	}

	if len(namespaceSelector.Include) > 0 {
		namespacesConditions := namespacesConditions(namespaceSelector.Include)
		includeNamespacesExpr := common.JoinWithAnd(
			// Ensure the k8s.namespace.name resource attribute is not nil,
			// so we don't drop logs without a namespace label
			common.ResourceAttributeIsNotNil(common.K8sNamespaceName),

			// Logs are dropped if the filter expression evaluates to true,
			// so we negate the match against included namespaces to keep only those
			common.Not(common.JoinWithOr(namespacesConditions...)),
		)
		filterExpressions = append(filterExpressions, includeNamespacesExpr)
	}

	return &FilterProcessor{
		Logs: FilterProcessorLogs{
			Log: filterExpressions,
		},
	}
}

func namespacesConditions(namespaces []string) []string {
	var conditions []string
	for _, ns := range namespaces {
		conditions = append(conditions, common.NamespaceEquals(ns))
	}

	return conditions
}

func dropIfInputSourceOTLPProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Logs: FilterProcessorLogs{
			Log: []string{
				// Drop all logs; the filter processor requires at least one valid condition expression,
				// to drop all logs, we use a condition that is always true for any log
				common.JoinWithOr(common.IsNotNil("log.observed_time"), common.IsNotNil("log.time")),
			},
		},
	}
}
