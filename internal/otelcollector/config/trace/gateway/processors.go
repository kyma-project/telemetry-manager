package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/servicename"
)

func makeProcessorsConfig() Processors {
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
			Sources: []config.PodAssociation{
				{
					From: "resource_attribute",
					Name: "k8s.pod.ip",
				},
			},
		},
		{
			Sources: []config.PodAssociation{
				{
					From: "resource_attribute",
					Name: "k8s.pod.uid",
				},
			},
		},
		{
			Sources: []config.PodAssociation{
				{
					From: "connection",
				},
			},
		},
	}

	return Processors{
		BaseProcessors: config.BaseProcessors{
			Batch: &config.BatchProcessor{
				SendBatchSize:    512,
				Timeout:          "10s",
				SendBatchMaxSize: 512,
			},
			MemoryLimiter: &config.MemoryLimiter{
				CheckInterval:        "1s",
				LimitPercentage:      60,
				SpikeLimitPercentage: 40,
			},
			K8sAttributes: &config.K8sAttributesProcessor{
				AuthType:    "serviceAccount",
				Passthrough: false,
				Extract: config.ExtractK8sMetadata{
					Metadata: k8sAttributes,
					Labels:   servicename.ExtractLabels(),
				},
				PodAssociation: podAssociations,
			},
			Resource: &config.ResourceProcessor{
				Attributes: []config.AttributeAction{
					{
						Action: "insert",
						Key:    "k8s.cluster.name",
						Value:  "${KUBERNETES_SERVICE_HOST}",
					},
				},
			},
		},
		SpanFilter: FilterProcessor{
			Traces: Traces{
				Span: makeSpanFilterConfig(),
			},
		},
		ResolveServiceName: makeResolveServiceNameConfig(),
	}
}
