package processors

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

func InsertClusterAttributesProcessorConfig(clusterName, cloudProvider string) *config.ResourceProcessor {
	if cloudProvider != "" {
		return &config.ResourceProcessor{
			Attributes: []config.AttributeAction{
				{
					Action: "insert",
					Key:    "k8s.cluster.name",
					Value:  clusterName,
				},
				{
					Action: "insert",
					Key:    "cloud.provider",
					Value:  cloudProvider,
				},
			},
		}
	}

	return &config.ResourceProcessor{
		Attributes: []config.AttributeAction{
			{
				Action: "insert",
				Key:    "k8s.cluster.name",
				Value:  clusterName,
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

func MakeResolveServiceNameConfig() *config.ServiceEnrichmentProcessor {
	return &config.ServiceEnrichmentProcessor{
		ResourceAttributes: []string{
			"kyma.kubernetes_io_app_name",
			"kyma.app_name"},
	}
}
