package agent

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
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
