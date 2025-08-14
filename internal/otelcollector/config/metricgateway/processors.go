package metricgateway

import (
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
)

func processorsConfig(opts BuildOptions) Processors {
	return Processors{
		BaseProcessors: common.BaseProcessors{
			Batch:         batchProcessorConfig(),
			MemoryLimiter: memoryLimiterProcessorConfig(),
		},
		K8sAttributes:                 common.K8sAttributesProcessorConfig(opts.Enrichments),
		InsertClusterAttributes:       common.InsertClusterAttributesProcessorConfig(opts.ClusterName, opts.ClusterUID, opts.CloudProvider),
		ResolveServiceName:            common.ResolveServiceNameConfig(),
		DropKymaAttributes:            common.DropKymaAttributesProcessorConfig(),
		DeleteSkipEnrichmentAttribute: deleteSkipEnrichmentAttributeProcessorConfig(),
		SetInstrumentationScopeKyma:   common.InstrumentationScopeProcessorConfig(opts.InstrumentationScopeVersion, common.InputSourceKyma),
		Dynamic:                       make(map[string]any),
	}
}

//nolint:mnd // hardcoded values
func batchProcessorConfig() *common.BatchProcessor {
	return &common.BatchProcessor{
		SendBatchSize:    1024,
		Timeout:          "10s",
		SendBatchMaxSize: 1024,
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

func deleteSkipEnrichmentAttributeProcessorConfig() *common.ResourceProcessor {
	return &common.ResourceProcessor{
		Attributes: []common.AttributeAction{
			{
				Action: "delete",
				Key:    common.SkipEnrichmentAttribute,
			},
		},
	}
}

func dropIfInputSourceRuntimeProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				common.ScopeNameEquals(common.InstrumentationScopeRuntime),
			},
		},
	}
}

func dropIfInputSourcePrometheusProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				common.ResourceAttributeEquals(common.KymaInputNameAttribute, common.KymaInputPrometheus),
			},
		},
	}
}

func dropIfInputSourceIstioProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				common.ScopeNameEquals(common.InstrumentationScopeIstio),
			},
		},
	}
}

func dropIfEnvoyMetricsDisabledProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				common.JoinWithAnd(common.IsMatch("name", "^envoy_.*"), common.ScopeNameEquals(common.InstrumentationScopeIstio)),
			},
		},
	}
}

func dropIfInputSourceOTLPProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				otlpInputSource(),
			},
		},
	}
}

func dropRuntimePodMetricsProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				common.JoinWithAnd(
					inputSourceEquals(common.InputSourceRuntime),
					common.IsMatch("name", "^k8s.pod.*"),
				),
			},
		},
	}
}

func dropRuntimeContainerMetricsProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				common.JoinWithAnd(
					inputSourceEquals(common.InputSourceRuntime),
					common.IsMatch("name", "(^k8s.container.*)|(^container.*)"),
				),
			},
		},
	}
}

func dropRuntimeNodeMetricsProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				common.JoinWithAnd(
					inputSourceEquals(common.InputSourceRuntime),
					common.IsMatch("name", "^k8s.node.*"),
				),
			},
		},
	}
}

func dropRuntimeVolumeMetricsProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				common.JoinWithAnd(
					inputSourceEquals(common.InputSourceRuntime),
					common.IsMatch("name", "^k8s.volume.*"),
				),
			},
		},
	}
}

func dropRuntimeDeploymentMetricsProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				common.JoinWithAnd(
					inputSourceEquals(common.InputSourceRuntime),
					common.IsMatch("name", "^k8s.deployment.*"),
				),
			},
		},
	}
}

func dropRuntimeStatefulSetMetricsProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				common.JoinWithAnd(
					inputSourceEquals(common.InputSourceRuntime),
					common.IsMatch("name", "^k8s.statefulset.*"),
				),
			},
		},
	}
}

func dropRuntimeDaemonSetMetricsProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				common.JoinWithAnd(
					inputSourceEquals(common.InputSourceRuntime),
					common.IsMatch("name", "^k8s.daemonset.*"),
				),
			},
		},
	}
}

func dropRuntimeJobMetricsProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				common.JoinWithAnd(
					inputSourceEquals(common.InputSourceRuntime),
					common.IsMatch("name", "^k8s.job.*"),
				),
			},
		},
	}
}

func filterByNamespaceProcessorConfig(namespaceSelector *telemetryv1alpha1.NamespaceSelector, inputSourceCondition string) *FilterProcessor {
	var filterExpressions []string

	if len(namespaceSelector.Exclude) > 0 {
		namespacesConditions := namespacesConditions(namespaceSelector.Exclude)

		// Drop metrics if the excluded namespaces are matched
		excludeNamespacesExpr := common.JoinWithAnd(inputSourceCondition, common.JoinWithOr(namespacesConditions...))
		filterExpressions = append(filterExpressions, excludeNamespacesExpr)
	}

	if len(namespaceSelector.Include) > 0 {
		namespacesConditions := namespacesConditions(namespaceSelector.Include)

		// metrics are dropped if the statement is true, so you need to negate the expression
		includeNamespacesExpr := common.JoinWithAnd(
			// Ensure we are filtering metrics from the correct input source
			inputSourceCondition,

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
		Metrics: FilterProcessorMetrics{
			Metric: filterExpressions,
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

func inputSourceEquals(inputSourceType common.InputSourceType) string {
	return common.ScopeNameEquals(common.InstrumentationScope[inputSourceType])
}

func otlpInputSource() string {
	// When instrumentation scope is not set to any of the following values
	// io.kyma-project.telemetry/runtime, io.kyma-project.telemetry/prometheus, io.kyma-project.telemetry/istio, and io.kyma-project.telemetry/kyma
	// we assume the metric is being pushed directly to metrics gateway.
	return fmt.Sprintf("not(%s or %s or %s or %s)",
		common.ScopeNameEquals(common.InstrumentationScopeRuntime),
		common.ResourceAttributeEquals(common.KymaInputNameAttribute, common.KymaInputPrometheus),
		common.ScopeNameEquals(common.InstrumentationScopeIstio),
		common.ScopeNameEquals(common.InstrumentationScopeKyma),
	)
}
