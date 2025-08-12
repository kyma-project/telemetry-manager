package common

func InsertClusterAttributesProcessorConfig(clusterName, clusterUID, cloudProvider string) *ResourceProcessor {
	if cloudProvider != "" {
		return &ResourceProcessor{
			Attributes: []AttributeAction{
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

	return &ResourceProcessor{
		Attributes: []AttributeAction{
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
		},
	}
}

func DropKymaAttributesProcessorConfig() *ResourceProcessor {
	return &ResourceProcessor{
		Attributes: []AttributeAction{
			{
				Action:       "delete",
				RegexPattern: "kyma.*",
			},
		},
	}
}

func ResolveServiceNameConfig() *ServiceEnrichmentProcessor {
	return &ServiceEnrichmentProcessor{
		ResourceAttributes: []string{
			kymaK8sIOAppName,
			kymaAppName,
		},
	}
}
