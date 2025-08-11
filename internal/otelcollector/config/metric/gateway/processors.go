package gateway

import (
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/ottlexpr"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/processors"
)

func processorsConfig(opts BuildOptions) Processors {
	return Processors{
		BaseProcessors: config.BaseProcessors{
			Batch:         batchProcessorConfig(),
			MemoryLimiter: memoryLimiterProcessorConfig(),
		},
		K8sAttributes:                 processors.K8sAttributesProcessorConfig(opts.Enrichments),
		InsertClusterAttributes:       processors.InsertClusterAttributesProcessorConfig(opts.ClusterName, opts.ClusterUID, opts.CloudProvider),
		ResolveServiceName:            processors.MakeResolveServiceNameConfig(),
		DropKymaAttributes:            processors.DropKymaAttributesProcessorConfig(),
		DeleteSkipEnrichmentAttribute: deleteSkipEnrichmentAttributeProcessorConfig(),
		SetInstrumentationScopeKyma:   metric.InstrumentationScopeProcessorConfig(opts.InstrumentationScopeVersion, metric.InputSourceKyma),
	}
}

//nolint:mnd // hardcoded values
func batchProcessorConfig() *config.BatchProcessor {
	return &config.BatchProcessor{
		SendBatchSize:    1024,
		Timeout:          "10s",
		SendBatchMaxSize: 1024,
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

func deleteSkipEnrichmentAttributeProcessorConfig() *config.ResourceProcessor {
	return &config.ResourceProcessor{
		Attributes: []config.AttributeAction{
			{
				Action: "delete",
				Key:    metric.SkipEnrichmentAttribute,
			},
		},
	}
}

func dropIfInputSourceRuntimeProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.ScopeNameEquals(metric.InstrumentationScopeRuntime),
			},
		},
	}
}

func dropIfInputSourcePrometheusProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.ResourceAttributeEquals(metric.KymaInputNameAttribute, metric.KymaInputPrometheus),
			},
		},
	}
}

func dropIfInputSourceIstioProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.ScopeNameEquals(metric.InstrumentationScopeIstio),
			},
		},
	}
}

func dropIfEnvoyMetricsDisabledProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(ottlexpr.IsMatch("name", "^envoy_.*"), ottlexpr.ScopeNameEquals(metric.InstrumentationScopeIstio)),
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
				ottlexpr.JoinWithAnd(
					inputSourceEquals(metric.InputSourceRuntime),
					ottlexpr.IsMatch("name", "^k8s.pod.*"),
				),
			},
		},
	}
}

func dropRuntimeContainerMetricsProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(
					inputSourceEquals(metric.InputSourceRuntime),
					ottlexpr.IsMatch("name", "(^k8s.container.*)|(^container.*)"),
				),
			},
		},
	}
}

func dropRuntimeNodeMetricsProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(
					inputSourceEquals(metric.InputSourceRuntime),
					ottlexpr.IsMatch("name", "^k8s.node.*"),
				),
			},
		},
	}
}

func dropRuntimeVolumeMetricsProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(
					inputSourceEquals(metric.InputSourceRuntime),
					ottlexpr.IsMatch("name", "^k8s.volume.*"),
				),
			},
		},
	}
}

func dropRuntimeDeploymentMetricsProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(
					inputSourceEquals(metric.InputSourceRuntime),
					ottlexpr.IsMatch("name", "^k8s.deployment.*"),
				),
			},
		},
	}
}

func dropRuntimeStatefulSetMetricsProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(
					inputSourceEquals(metric.InputSourceRuntime),
					ottlexpr.IsMatch("name", "^k8s.statefulset.*"),
				),
			},
		},
	}
}

func dropRuntimeDaemonSetMetricsProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(
					inputSourceEquals(metric.InputSourceRuntime),
					ottlexpr.IsMatch("name", "^k8s.daemonset.*"),
				),
			},
		},
	}
}

func dropRuntimeJobMetricsProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(
					inputSourceEquals(metric.InputSourceRuntime),
					ottlexpr.IsMatch("name", "^k8s.job.*"),
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
		excludeNamespacesExpr := ottlexpr.JoinWithAnd(inputSourceCondition, ottlexpr.JoinWithOr(namespacesConditions...))
		filterExpressions = append(filterExpressions, excludeNamespacesExpr)
	}

	if len(namespaceSelector.Include) > 0 {
		namespacesConditions := namespacesConditions(namespaceSelector.Include)

		// metrics are dropped if the statement is true, so you need to negate the expression
		includeNamespacesExpr := ottlexpr.JoinWithAnd(
			// Ensure we are filtering metrics from the correct input source
			inputSourceCondition,

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
		Metrics: FilterProcessorMetrics{
			Metric: filterExpressions,
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

func inputSourceEquals(inputSourceType metric.InputSourceType) string {
	return ottlexpr.ScopeNameEquals(metric.InstrumentationScope[inputSourceType])
}

func otlpInputSource() string {
	// When instrumentation scope is not set to any of the following values
	// io.kyma-project.telemetry/runtime, io.kyma-project.telemetry/prometheus, io.kyma-project.telemetry/istio, and io.kyma-project.telemetry/kyma
	// we assume the metric is being pushed directly to metrics gateway.
	return fmt.Sprintf("not(%s or %s or %s or %s)",
		ottlexpr.ScopeNameEquals(metric.InstrumentationScopeRuntime),
		ottlexpr.ResourceAttributeEquals(metric.KymaInputNameAttribute, metric.KymaInputPrometheus),
		ottlexpr.ScopeNameEquals(metric.InstrumentationScopeIstio),
		ottlexpr.ScopeNameEquals(metric.InstrumentationScopeKyma),
	)
}
