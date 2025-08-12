package metricagent

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
)

func processorsConfig(inputs inputSources, instrumentationScopeVersion string) Processors {
	pc := Processors{
		BaseProcessors: common.BaseProcessors{
			Batch:         batchProcessorConfig(),
			MemoryLimiter: memoryLimiterProcessorConfig(),
		},
		DropVirtualNetworkInterfaces: dropVirtualNetworkInterfacesProcessorConfig(),
	}

	if inputs.runtime || inputs.prometheus || inputs.istio {
		pc.DeleteServiceName = deleteServiceNameProcessorConfig()

		if inputs.runtime {
			pc.SetInstrumentationScopeRuntime = common.InstrumentationScopeProcessorConfig(instrumentationScopeVersion, common.InputSourceRuntime, common.InputSourceK8sCluster)
			pc.InsertSkipEnrichmentAttribute = insertSkipEnrichmentAttributeProcessorConfig()

			if inputs.runtimeResources.volume {
				pc.DropNonPVCVolumesMetrics = dropNonPVCVolumesMetricsProcessorConfig()
			}
		}

		if inputs.prometheus {
			pc.SetInstrumentationScopePrometheus = common.InstrumentationScopeProcessorConfig(instrumentationScopeVersion, common.InputSourcePrometheus)
		}

		if inputs.istio {
			pc.IstioNoiseFilter = &common.IstioNoiseFilterProcessor{}
			pc.SetInstrumentationScopeIstio = common.InstrumentationScopeProcessorConfig(instrumentationScopeVersion, common.InputSourceIstio)
		}
	}

	return pc
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

func deleteServiceNameProcessorConfig() *common.ResourceProcessor {
	return &common.ResourceProcessor{
		Attributes: []common.AttributeAction{
			{
				Action: "delete",
				Key:    "service.name",
			},
		},
	}
}

func insertSkipEnrichmentAttributeProcessorConfig() *common.TransformProcessor {
	metricsToSkipEnrichment := []string{
		"node",
		"statefulset",
		"daemonset",
		"deployment",
		"job",
	}

	return common.MetricTransformProcessor([]common.TransformProcessorStatements{{
		Conditions: metricNameConditionsWithIsMatch(metricsToSkipEnrichment),
		Statements: []string{fmt.Sprintf("set(resource.attributes[\"%s\"], \"true\")", common.SkipEnrichmentAttribute)},
	}})
}

func dropNonPVCVolumesMetricsProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				common.JoinWithAnd(
					// identify volume metrics by checking existence of "k8s.volume.name" resource attribute
					common.ResourceAttributeIsNotNil("k8s.volume.name"),
					common.ResourceAttributeNotEquals("k8s.volume.type", "persistentVolumeClaim"),
				),
			},
		},
	}
}

func metricNameConditionsWithIsMatch(metrics []string) []string {
	var conditions []string

	for _, m := range metrics {
		condition := common.IsMatch("metric.name", fmt.Sprintf("^k8s.%s.*", m))
		conditions = append(conditions, condition)
	}

	return conditions
}

func dropVirtualNetworkInterfacesProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Datapoint: []string{
				common.JoinWithAnd(
					common.IsMatch("metric.name", "^k8s.node.network.*"),
					common.Not(common.IsMatch("attributes[\"interface\"]", "^(eth|en).*")),
				),
			},
		},
	}
}
