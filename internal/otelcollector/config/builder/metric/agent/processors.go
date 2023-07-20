package agent

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/metric"
)

func makeProcessorsConfig(inputs inputSources) ProcessorsConfig {
	var processorsConfig ProcessorsConfig

	if inputs.runtime || inputs.workloads {
		processorsConfig.DeleteServiceName = makeDeleteServiceNameConfig()

		if inputs.runtime {
			processorsConfig.InsertInputSourceRuntime = makeEmittedByConfig(metric.InputSourceRuntime)
		}

		if inputs.workloads {
			processorsConfig.InsertInputSourceWorkloads = makeEmittedByConfig(metric.InputSourceWorkloads)
		}
	}

	return processorsConfig
}

func makeDeleteServiceNameConfig() *common.ResourceProcessorConfig {
	return &common.ResourceProcessorConfig{
		Attributes: []common.AttributeAction{
			{
				Action: "delete",
				Key:    "service.name",
			},
		},
	}
}

func makeEmittedByConfig(inputSource metric.InputSourceType) *common.ResourceProcessorConfig {
	return &common.ResourceProcessorConfig{
		Attributes: []common.AttributeAction{
			{
				Action: "insert",
				Key:    metric.InputSourceAttribute,
				Value:  string(inputSource),
			},
		},
	}
}
