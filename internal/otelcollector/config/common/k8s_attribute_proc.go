package common

import (
	"fmt"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
)

const (
	kymaK8sIOAppName = "kyma.kubernetes_io_app_name"
	kymaAppName      = "kyma.app_name"
)

type PodLabel struct {
	Key       string
	KeyPrefix string
}

func K8sAttributesProcessorConfig(enrichments *operatorv1alpha1.EnrichmentSpec) *K8sAttributesProcessor {
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

	podAssociations := []PodAssociations{
		{
			Sources: []PodAssociation{{From: "resource_attribute", Name: "k8s.pod.ip"}},
		},
		{
			Sources: []PodAssociation{{From: "resource_attribute", Name: "k8s.pod.uid"}},
		},
		{
			Sources: []PodAssociation{{From: "connection"}},
		},
	}

	return &K8sAttributesProcessor{
		AuthType:    "serviceAccount",
		Passthrough: false,
		Extract: ExtractK8sMetadata{
			Metadata: k8sAttributes,
			Labels:   append(extractLabels(), buildExtractPodLabels(enrichments)...),
		},
		PodAssociation: podAssociations,
	}
}

func extractLabels() []ExtractLabel {
	return []ExtractLabel{
		{
			From:    "pod",
			Key:     "app.kubernetes.io/name",
			TagName: kymaK8sIOAppName,
		},
		{
			From:    "pod",
			Key:     "app",
			TagName: kymaAppName,
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

func buildExtractPodLabels(enrichments *operatorv1alpha1.EnrichmentSpec) []ExtractLabel {
	extractPodLabels := make([]ExtractLabel, 0)

	if enrichments != nil && len(enrichments.ExtractPodLabels) > 0 {
		for _, label := range enrichments.ExtractPodLabels {
			labelConfig := ExtractLabel{
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
