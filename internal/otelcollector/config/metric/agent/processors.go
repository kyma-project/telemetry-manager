package agent

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
)

func makeProcessorsConfig(inputs inputSources) Processors {
	processorsConfig := Processors{
		BaseProcessors: config.BaseProcessors{
			Batch:         makeBatchProcessorConfig(),
			MemoryLimiter: makeMemoryLimiterConfig(),
		},
	}

	if inputs.runtime || inputs.prometheus || inputs.istio {
		processorsConfig.DeleteServiceName = makeDeleteServiceNameConfig()

		if inputs.runtime {
			processorsConfig.InsertInputSourceRuntime = makeEmittedByConfig(metric.InputSourceRuntime)
		}

		if inputs.prometheus {
			processorsConfig.InsertInputSourcePrometheus = makeEmittedByConfig(metric.InputSourcePrometheus)
		}

		if inputs.istio {
			processorsConfig.InsertInputSourceIstio = makeEmittedByConfig(metric.InputSourceIstio)
			processorsConfig.DropInternalCommunication = makeFilterToDropMetricsForTelemetryComponents()
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
		CheckInterval:        "0.1s",
		LimitPercentage:      75,
		SpikeLimitPercentage: 20,
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

func makeEmittedByConfig(inputSource metric.InputSourceType) *config.ResourceProcessor {
	return &config.ResourceProcessor{
		Attributes: []config.AttributeAction{
			{
				Action: "insert",
				Key:    metric.InputSourceAttribute,
				Value:  string(inputSource),
			},
		},
	}
}
