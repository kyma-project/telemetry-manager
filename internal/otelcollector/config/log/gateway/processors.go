package gateway

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/ottlexpr"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/processors"
)

func processorsConfig(opts BuildOptions) Processors {
	return Processors{
		BaseProcessors: config.BaseProcessors{
			Batch:         batchProcessorConfig(),
			MemoryLimiter: memoryLimiterProcessorConfig(),
		},
		SetObsTimeIfZero:        setObsTimeIfZeroProcessorConfig(),
		IstioNoiseFilter:        &config.IstioNoiseFilterProcessor{},
		K8sAttributes:           processors.K8sAttributesProcessorConfig(opts.Enrichments),
		InsertClusterAttributes: processors.InsertClusterAttributesProcessorConfig(opts.ClusterName, opts.ClusterUID, opts.CloudProvider),
		ResolveServiceName:      processors.MakeResolveServiceNameConfig(),
		DropKymaAttributes:      processors.DropKymaAttributesProcessorConfig(),
		IstioEnrichment:         istioEnrichmentProcessorConfig(opts),
	}
}

func istioEnrichmentProcessorConfig(opts BuildOptions) *IstioEnrichmentProcessor {
	return &IstioEnrichmentProcessor{
		ScopeVersion: opts.ModuleVersion,
	}
}

//nolint:mnd // hardcoded values
func batchProcessorConfig() *config.BatchProcessor {
	return &config.BatchProcessor{
		SendBatchSize:    512,
		Timeout:          "10s",
		SendBatchMaxSize: 512,
	}
}

//nolint:mnd // hardcoded values
func memoryLimiterProcessorConfig() *config.MemoryLimiter {
	return &config.MemoryLimiter{
		CheckInterval:        "1s",
		LimitPercentage:      75,
		SpikeLimitPercentage: 15,
	}
}

func setObsTimeIfZeroProcessorConfig() *config.TransformProcessor {
	return &config.TransformProcessor{
		ErrorMode: "ignore",
		LogStatements: []config.TransformProcessorStatements{
			{
				Conditions: []string{
					"log.observed_time_unix_nano == 0",
				},
				Statements: []string{"set(log.observed_time, Now())"},
			},
		},
	}
}

func namespaceFilterProcessorConfig(namespaceSelector *telemetryv1alpha1.NamespaceSelector) *FilterProcessor {
	var filterExpressions []string

	if len(namespaceSelector.Exclude) > 0 {
		namespacesConditions := namespacesConditions(namespaceSelector.Exclude)

		// Drop logs if the excluded namespaces are matched
		excludeNamespacesExpr := ottlexpr.JoinWithOr(namespacesConditions...)
		filterExpressions = append(filterExpressions, excludeNamespacesExpr)
	}

	if len(namespaceSelector.Include) > 0 {
		namespacesConditions := namespacesConditions(namespaceSelector.Include)
		includeNamespacesExpr := ottlexpr.JoinWithAnd(
			// Ensure the k8s.namespace.name resource attribute is not nil,
			// so we don't drop logs without a namespace label
			ottlexpr.ResourceAttributeIsNotNil(ottlexpr.K8sNamespaceName),

			// Logs are dropped if the filter expression evaluates to true,
			// so we negate the match against included namespaces to keep only those
			ottlexpr.Not(ottlexpr.JoinWithOr(namespacesConditions...)),
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
		conditions = append(conditions, ottlexpr.NamespaceEquals(ns))
	}

	return conditions
}

func dropIfInputSourceOTLPProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Logs: FilterProcessorLogs{
			Log: []string{
				// Drop all logs; the filter processor requires at least one valid condition expression,
				// to drop all logs, we use a condition that is always true for any log
				ottlexpr.JoinWithOr(ottlexpr.IsNotNil("log.observed_time"), ottlexpr.IsNotNil("log.time")),
			},
		},
	}
}
