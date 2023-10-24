package gatewayprocs

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

func InsertClusterNameProcessorConfig() *config.ResourceProcessor {
	return &config.ResourceProcessor{
		Attributes: []config.AttributeAction{
			{
				Action: "insert",
				Key:    "k8s.cluster.name",
				Value:  "${KUBERNETES_SERVICE_HOST}",
			},
		},
	}
}

func DropKymaAttributesProcessorConfig() *config.ResourceProcessor {
	return &config.ResourceProcessor{
		Attributes: []config.AttributeAction{
			{
				Action:       "delete",
				RegexPattern: "kyma.*",
			},
		},
	}
}
