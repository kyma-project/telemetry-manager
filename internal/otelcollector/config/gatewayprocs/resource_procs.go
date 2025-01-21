package gatewayprocs

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

func InsertClusterAttributesProcessorConfig() *config.ResourceProcessor {
	return &config.ResourceProcessor{
		Attributes: []config.AttributeAction{
			{
				Action: "insert",
				Key:    "k8s.cluster.name",
				Value:  "${CLUSTER_NAME}",
			},
			{
				Action: "insert",
				Key:    "cloud.provider",
				Value:  "${CLOUD_PROVIDER}",
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
