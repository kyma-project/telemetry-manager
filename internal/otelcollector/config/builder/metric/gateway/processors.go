package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

func makeProcessorsConfig() config.ProcessorsConfig {
	return config.ProcessorsConfig{
		Batch:         makeBatchProcessorConfig(),
		MemoryLimiter: makeMemoryLimiterConfig(),
		K8sAttributes: makeK8sAttributesProcessorConfig(),
		Resource:      makeResourceProcessorConfig(),
		Transform:     makeTransformProcessorConfig(),
	}
}

func makeBatchProcessorConfig() *config.BatchProcessorConfig {
	return &config.BatchProcessorConfig{
		SendBatchSize:    1024,
		Timeout:          "10s",
		SendBatchMaxSize: 1024,
	}
}

func makeMemoryLimiterConfig() *config.MemoryLimiterConfig {
	return &config.MemoryLimiterConfig{
		CheckInterval:        "1s",
		LimitPercentage:      75,
		SpikeLimitPercentage: 10,
	}
}

func makeK8sAttributesProcessorConfig() *config.K8sAttributesProcessorConfig {
	k8sAttributes := []string{
		"k8s.pod.name",
		"k8s.node.name",
		"k8s.namespace.name",
		"k8s.deployment.name",
		"k8s.statefulset.name",
		"k8s.daemonset.name",
		"k8s.cronjob.name",
		"k8s.job.name",
	}

	podAssociations := []config.PodAssociations{
		{
			Sources: []config.PodAssociation{{From: "resource_attribute", Name: "k8s.pod.ip"}},
		},
		{
			Sources: []config.PodAssociation{{From: "resource_attribute", Name: "k8s.pod.uid"}},
		},
		{
			Sources: []config.PodAssociation{{From: "connection"}},
		},
	}

	return &config.K8sAttributesProcessorConfig{
		AuthType:    "serviceAccount",
		Passthrough: false,
		Extract: config.ExtractK8sMetadataConfig{
			Metadata: k8sAttributes,
		},
		PodAssociation: podAssociations,
	}
}

func makeResourceProcessorConfig() *config.ResourceProcessorConfig {
	return &config.ResourceProcessorConfig{
		Attributes: []config.AttributeAction{
			{
				Action: "insert",
				Key:    "k8s.cluster.name",
				Value:  "${KUBERNETES_SERVICE_HOST}",
			},
		},
	}
}

func makeTransformProcessorConfig() *config.TransformProcessorConfig {
	return &config.TransformProcessorConfig{}
}
