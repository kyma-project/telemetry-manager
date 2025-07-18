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
		DropVirtualNetworkInterfaces: makeDropVirtualNetworkInterfacesProcessor(),
	}

	if inputs.runtime || inputs.prometheus || inputs.istio {
		processorsConfig.DeleteServiceName = makeDeleteServiceNameConfig()

		if inputs.runtime {
			processorsConfig.SetInstrumentationScopeRuntime = metric.MakeInstrumentationScopeProcessor(instrumentationScopeVersion, metric.InputSourceRuntime, metric.InputSourceK8sCluster)
			processorsConfig.InsertSkipEnrichmentAttribute = makeInsertSkipEnrichmentAttributeProcessor()

			if inputs.runtimeResources.volume {
				processorsConfig.DropNonPVCVolumesMetrics = makeDropNonPVCVolumesMetricsProcessor()
			}
		}

		if inputs.prometheus {
			processorsConfig.SetInstrumentationScopePrometheus = metric.MakeInstrumentationScopeProcessor(instrumentationScopeVersion, metric.InputSourcePrometheus)
		}

		if inputs.istio {
			processorsConfig.IstioNoiseFilter = &config.IstioNoiseFilterProcessor{}
			processorsConfig.SetInstrumentationScopeIstio = metric.MakeInstrumentationScopeProcessor(instrumentationScopeVersion, metric.InputSourceIstio)
		}
	}

	return processorsConfig
}

//nolint:mnd // hardcoded values
func makeBatchProcessorConfig() *config.BatchProcessor {
	return &config.BatchProcessor{
		SendBatchSize:    1024,
		Timeout:          "10s",
		SendBatchMaxSize: 1024,
	}
}

//nolint:mnd // hardcoded values
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
	metricsToSkipEnrichment := []string{
		"node",
		"statefulset",
		"daemonset",
		"deployment",
		"job",
	}

	return &metric.TransformProcessor{
		ErrorMode: "ignore",
		MetricStatements: []config.TransformProcessorStatements{
			{
				Statements: []string{
					fmt.Sprintf("set(resource.attributes[\"%s\"], \"true\")", metric.SkipEnrichmentAttribute),
				},
				Conditions: makeMetricNameConditionsWithIsMatch(metricsToSkipEnrichment),
			},
		},
	}
}

func makeDropNonPVCVolumesMetricsProcessor() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(
					// identify volume metrics by checking existence of "k8s.volume.name" resource attribute
					ottlexpr.ResourceAttributeIsNotNil("k8s.volume.name"),
					ottlexpr.ResourceAttributeNotEquals("k8s.volume.type", "persistentVolumeClaim"),
				),
			},
		},
	}
}

func makeMetricNameConditionsWithIsMatch(metrics []string) []string {
	var conditions []string

	for _, m := range metrics {
		condition := ottlexpr.IsMatch("metric.name", fmt.Sprintf("^k8s.%s.*", m))
		conditions = append(conditions, condition)
	}

	return conditions
}

func makeDropVirtualNetworkInterfacesProcessor() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Datapoint: []string{
				ottlexpr.JoinWithAnd(
					ottlexpr.IsMatch("metric.name", "^k8s.node.network.*"),
					ottlexpr.Not(ottlexpr.IsMatch("attributes[\"interface\"]", "^(eth|en).*")),
				),
			},
		},
	}
}
