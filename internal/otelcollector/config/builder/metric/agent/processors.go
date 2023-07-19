package agent

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common"
)

func makeProcessorsConfig(inputs inputSources) ProcessorsConfig {
	var processorsConfig ProcessorsConfig

	if inputs.runtime || inputs.workloads {
		processorsConfig.DeleteServiceName = makeDeleteServiceNameConfig()

		if inputs.runtime {
			processorsConfig.InsertInputSourceRuntime = makeEmittedByConfig("runtime")
		}

		if inputs.workloads {
			processorsConfig.InsertInputSourceWorkloads = makeEmittedByConfig("workloads")
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

func makeEmittedByConfig(value string) *common.ResourceProcessorConfig {
	return &common.ResourceProcessorConfig{
		Attributes: []common.AttributeAction{
			{
				Action: "insert",
				Key:    "kyma.source",
				Value:  value,
			},
		},
	}
}
