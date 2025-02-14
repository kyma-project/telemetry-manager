package gatewayprocs

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

type Enrichments struct {
	Enabled   bool
	PodLabels []PodLabel
}

type PodLabel struct {
	Key       string
	KeyPrefix string
}

func K8sAttributesProcessorConfig(enrichments Enrichments) *config.K8sAttributesProcessor {
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

	return &config.K8sAttributesProcessor{
		AuthType:    "serviceAccount",
		Passthrough: false,
		Extract: config.ExtractK8sMetadata{
			Metadata: k8sAttributes,
			Labels:   append(extractLabels(), buildExtractPodLabels(enrichments)...),
		},
		PodAssociation: podAssociations,
	}
}

func extractLabels() []config.ExtractLabel {
	return []config.ExtractLabel{
		{
			From:    "pod",
			Key:     "app.kubernetes.io/name",
			TagName: "kyma.kubernetes_io_app_name",
		},
		{
			From:    "pod",
			Key:     "app",
			TagName: "kyma.app_name",
		},
		{
			From:    "node",
			Key:     "topology.kubernetes.io/region",
			TagName: "cloud.region",
		},
		{
			From:    "node",
			Key:     "topology.kubernetes.io/zone",
			TagName: "cloud.availability_zone",
		},
		{
			From:    "node",
			Key:     "node.kubernetes.io/instance-type",
			TagName: "host.type",
		},
		{
			From:    "node",
			Key:     "kubernetes.io/arch",
			TagName: "host.arch",
		},
	}
}

func buildExtractPodLabels(enrichments Enrichments) []config.ExtractLabel {
	extractPodLabels := make([]config.ExtractLabel, 0)

	if enrichments.Enabled {
		for _, label := range enrichments.PodLabels {
			labelConfig := config.ExtractLabel{
				From:    "pod",
				TagName: "k8s.pod.label.$0",
			}

			if label.KeyPrefix != "" {
				labelConfig.KeyRegex = fmt.Sprintf("(%s.*)", label.KeyPrefix)
			} else {
				labelConfig.KeyRegex = fmt.Sprintf("(^%s$)", label.Key)
			}

			extractPodLabels = append(extractPodLabels, labelConfig)
		}
	}

	return extractPodLabels
}
