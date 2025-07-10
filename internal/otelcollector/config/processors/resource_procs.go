package processors

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

func InsertClusterAttributesProcessorConfig(clusterName, clusterUID, cloudProvider string) *config.ResourceProcessor {
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
					Key:    "k8s.cluster.uid",
					Value:  clusterUID,
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
			{
				Action: "insert",
				Key:    "k8s.cluster.uid",
				Value:  "",
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
			kymaK8sIOAppName,
			kymaAppName,
		},
	}
}
