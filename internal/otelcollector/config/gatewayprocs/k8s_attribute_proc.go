package gatewayprocs

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

func K8sAttributesProcessorConfig() *config.K8sAttributesProcessor {
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
			Labels:   extractLabels(),
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
	}
}
