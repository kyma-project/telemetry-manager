package agent

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

func makeProcessorsConfig() ProcessorsConfig {
	return ProcessorsConfig{
		DropServiceName: &config.ResourceProcessorConfig{
			Attributes: []config.AttributeAction{
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

func makeEmittedByConfig(value string) *config.ResourceProcessorConfig {
	return &config.ResourceProcessorConfig{
		Attributes: []config.AttributeAction{
			{
				Action: "insert",
				Key:    "kyma.source",
				Value:  value,
			},
		},
	}
}
