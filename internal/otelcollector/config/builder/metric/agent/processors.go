package agent

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

func makeProcessorsConfig() config.ProcessorsConfig {
	return config.ProcessorsConfig{
		Resource: &config.ResourceProcessorConfig{
			Attributes: []config.AttributeAction{
				{
					Action: "delete",
					Key:    "service.name",
				},
			},
		},
	}
}
