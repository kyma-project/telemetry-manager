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
			processorsConfig.SetInstrumentationScopeRuntime = metric.MakeInstrumentationScopeProcessor(metric.InputSourceRuntime, instrumentationScopeVersion)
			processorsConfig.InsertSkipEnrichmentAttribute = makeInsertSkipEnrichmentAttributeProcessor()
			if inputs.runtimeResources.volume {
				processorsConfig.DropNonPVCVolumesMetrics = makeDropNonPVCVolumesMetricsProcessor()
			}
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

func makeDropNonPVCVolumesMetricsProcessor() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(
					// identify volume metrics by checking existence of "k8s.volume.name" resource attribute
					ottlexpr.ResourceAttributeNotNil("k8s.volume.name"),
					ottlexpr.ResourceAttributeNotEquals("k8s.volume.type", "persistentVolumeClaim"),
				),
			},
		},
	}
}
