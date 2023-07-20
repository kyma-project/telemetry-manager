package gateway

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/metric"
)

func makeProcessorsConfig() ProcessorsConfig {
	return ProcessorsConfig{
		BaseProcessorsConfig: common.BaseProcessorsConfig{
			Batch:         makeBatchProcessorConfig(),
			MemoryLimiter: makeMemoryLimiterConfig(),
			K8sAttributes: makeK8sAttributesProcessorConfig(),
			Resource:      makeResourceProcessorConfig(),
		},
	}
}

func makeBatchProcessorConfig() *common.BatchProcessorConfig {
	return &common.BatchProcessorConfig{
		SendBatchSize:    1024,
		Timeout:          "10s",
		SendBatchMaxSize: 1024,
	}
}

func makeMemoryLimiterConfig() *common.MemoryLimiterConfig {
	return &common.MemoryLimiterConfig{
		CheckInterval:        "1s",
		LimitPercentage:      75,
		SpikeLimitPercentage: 10,
	}
}

func makeK8sAttributesProcessorConfig() *common.K8sAttributesProcessorConfig {
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

	podAssociations := []common.PodAssociations{
		{
			Sources: []common.PodAssociation{{From: "resource_attribute", Name: "k8s.pod.ip"}},
		},
		{
			Sources: []common.PodAssociation{{From: "resource_attribute", Name: "k8s.pod.uid"}},
		},
		{
			Sources: []common.PodAssociation{{From: "connection"}},
		},
	}

	return &common.K8sAttributesProcessorConfig{
		AuthType:    "serviceAccount",
		Passthrough: false,
		Extract: common.ExtractK8sMetadataConfig{
			Metadata: k8sAttributes,
		},
		PodAssociation: podAssociations,
	}
}

func makeResourceProcessorConfig() *common.ResourceProcessorConfig {
	return &common.ResourceProcessorConfig{
		Attributes: []common.AttributeAction{
			{
				Action: "insert",
				Key:    "k8s.cluster.name",
				Value:  "${KUBERNETES_SERVICE_HOST}",
			},
		},
	}
}

func makeDropIfInputSourceRuntimeConfig() *FilterProcessorConfig {
	return &FilterProcessorConfig{
		Metrics: FilterProcessorMetricConfig{
			DataPoint: []string{
				fmt.Sprintf("resource.attributes[\"%s\"] == \"%s\"", metric.InputSourceAttribute, metric.InputSourceRuntime),
			},
		},
	}
}

func makeDropIfInputSourceWorkloadsConfig() *FilterProcessorConfig {
	return &FilterProcessorConfig{
		Metrics: FilterProcessorMetricConfig{
			DataPoint: []string{
				fmt.Sprintf("resource.attributes[\"%s\"] == \"%s\"", metric.InputSourceAttribute, metric.InputSourceWorkloads),
			},
		},
	}
}
