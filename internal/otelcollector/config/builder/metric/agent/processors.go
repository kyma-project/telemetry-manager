package agent

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common"
)

func makeProcessorsConfig() ProcessorsConfig {
	return ProcessorsConfig{
		DropServiceName: &common.ResourceProcessorConfig{
			Attributes: []common.AttributeAction{
				{
					Action: "delete",
					Key:    "service.name",
				},
			},
		},
		EmittedByRuntime:   makeEmittedByConfig("runtime"),
		EmittedByWorkloads: makeEmittedByConfig("workloads"),
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
