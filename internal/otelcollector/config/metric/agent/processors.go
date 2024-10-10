package agent

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/ottlexpr"
)

func makeProcessorsConfig(inputs inputSources, instrumentationScopeVersion string) Processors {
	processorsConfig := Processors{
		BaseProcessors: config.BaseProcessors{
			Batch:         makeBatchProcessorConfig(),
			MemoryLimiter: makeMemoryLimiterConfig(),
		},
	}

	if inputs.runtime || inputs.prometheus || inputs.istio {
		processorsConfig.DeleteServiceName = makeDeleteServiceNameConfig()

		if inputs.runtime {
			processorsConfig.SetInstrumentationScopeRuntime = metric.MakeInstrumentScopeRuntime(instrumentationScopeVersion, metric.InputSourceRuntime, metric.InputSourceK8sCluster)
			processorsConfig.InsertSkipEnrichmentAttribute = makeInsertSkipEnrichmentAttributeProcessor()
			processorsConfig.DropK8sClusterMetrics = makeK8sClusterDropMetrics()
		}

		if inputs.prometheus {
			processorsConfig.SetInstrumentationScopePrometheus = metric.MakeInstrumentationScopeProcessor(metric.InputSourcePrometheus, instrumentationScopeVersion)
		}

		if inputs.istio {
			processorsConfig.DropInternalCommunication = makeFilterToDropMetricsForTelemetryComponents()
			processorsConfig.SetInstrumentationScopeIstio = metric.MakeInstrumentationScopeProcessor(metric.InputSourceIstio, instrumentationScopeVersion)
		}
	}

	return processorsConfig
}

func makeBatchProcessorConfig() *config.BatchProcessor {
	return &config.BatchProcessor{
		SendBatchSize:    1024,
		Timeout:          "10s",
		SendBatchMaxSize: 1024,
	}
}

func makeMemoryLimiterConfig() *config.MemoryLimiter {
	return &config.MemoryLimiter{
		CheckInterval:        "1s",
		LimitPercentage:      75,
		SpikeLimitPercentage: 15,
	}
}

func makeDeleteServiceNameConfig() *config.ResourceProcessor {
	return &config.ResourceProcessor{
		Attributes: []config.AttributeAction{
			{
				Action: "delete",
				Key:    "service.name",
			},
		},
	}
}

func makeInsertSkipEnrichmentAttributeProcessor() *metric.TransformProcessor {
	return &metric.TransformProcessor{
		ErrorMode: "ignore",
		MetricStatements: []config.TransformProcessorStatements{
			{
				Context: "metric",
				Statements: []string{
					fmt.Sprintf("set(resource.attributes[\"%s\"], \"true\")", metric.SkipEnrichmentAttribute),
				},
				Conditions: []string{
					ottlexpr.IsMatch("name", "^k8s.node.*"),
				},
			},
		},
	}
}

// Drop the metrics scraped by k8s cluster which are not workload related, So all besides the pod and container metrics
// Complete list of the metrics is here: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/k8sclusterreceiver/documentation.md
func makeK8sClusterDropMetrics() *FilterProcessor {
	metricNames := []string{
		"deployment",
		"cronjob",
		"daemonset",
		"hpa",
		"job",
		"replicaset",
		"resource_quota",
		"statefulset",
	}

	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(
					ottlexpr.ScopeNameEquals(metric.InstrumentationScope[metric.InputSourceRuntime]),
					ottlexpr.IsMatch("name", fmt.Sprintf("^k8s.%s.*", ottlexpr.JoinWithRegExpOr(metricNames...))),
				),
			},
		},
	}
}
