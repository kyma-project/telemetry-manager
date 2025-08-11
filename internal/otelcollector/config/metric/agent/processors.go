package agent

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/ottlexpr"
)

func processorsConfig(inputs inputSources, instrumentationScopeVersion string) Processors {
	pc := Processors{
		BaseProcessors: config.BaseProcessors{
			Batch:         batchProcessorConfig(),
			MemoryLimiter: memoryLimiterProcessorConfig(),
		},
		DropVirtualNetworkInterfaces: dropVirtualNetworkInterfacesProcessorConfig(),
	}

	if inputs.runtime || inputs.prometheus || inputs.istio {
		pc.DeleteServiceName = deleteServiceNameProcessorConfig()

		if inputs.runtime {
			pc.SetInstrumentationScopeRuntime = metric.InstrumentationScopeProcessorConfig(instrumentationScopeVersion, metric.InputSourceRuntime, metric.InputSourceK8sCluster)
			pc.InsertSkipEnrichmentAttribute = insertSkipEnrichmentAttributeProcessorConfig()

			if inputs.runtimeResources.volume {
				pc.DropNonPVCVolumesMetrics = dropNonPVCVolumesMetricsProcessorConfig()
			}
		}

		if inputs.prometheus {
			pc.SetInstrumentationScopePrometheus = metric.InstrumentationScopeProcessorConfig(instrumentationScopeVersion, metric.InputSourcePrometheus)
		}

		if inputs.istio {
			pc.IstioNoiseFilter = &config.IstioNoiseFilterProcessor{}
			pc.SetInstrumentationScopeIstio = metric.InstrumentationScopeProcessorConfig(instrumentationScopeVersion, metric.InputSourceIstio)
		}
	}

	return pc
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

func deleteServiceNameProcessorConfig() *config.ResourceProcessor {
	return &config.ResourceProcessor{
		Attributes: []config.AttributeAction{
			{
				Action: "delete",
				Key:    "service.name",
			},
		},
	}
}

func insertSkipEnrichmentAttributeProcessorConfig() *metric.TransformProcessor {
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
				Conditions: metricNameConditionsWithIsMatch(metricsToSkipEnrichment),
			},
		},
	}
}

func dropNonPVCVolumesMetricsProcessorConfig() *FilterProcessor {
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

func metricNameConditionsWithIsMatch(metrics []string) []string {
	var conditions []string

	for _, m := range metrics {
		condition := ottlexpr.IsMatch("metric.name", fmt.Sprintf("^k8s.%s.*", m))
		conditions = append(conditions, condition)
	}

	return conditions
}

func dropVirtualNetworkInterfacesProcessorConfig() *FilterProcessor {
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
