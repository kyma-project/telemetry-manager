package gatewayprocs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

func TestK8sAttributesProcessorConfig(t *testing.T) {
	require := require.New(t)

	expectedPodAssociations := []config.PodAssociations{
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
	expectedK8sAttributes := []string{
		"k8s.pod.name",
		"k8s.node.name",
		"k8s.namespace.name",
		"k8s.deployment.name",
		"k8s.statefulset.name",
		"k8s.daemonset.name",
		"k8s.cronjob.name",
		"k8s.job.name",
	}
	expectedExtractLabels := []config.ExtractLabel{
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

	config := K8sAttributesProcessorConfig()

	require.Equal("serviceAccount", config.AuthType)
	require.Equal(false, config.Passthrough)
	require.Equal(expectedPodAssociations, config.PodAssociation, "PodAssociation should match")

	require.ElementsMatch(expectedK8sAttributes, config.Extract.Metadata, "Metadata should match")
	require.ElementsMatch(expectedExtractLabels, config.Extract.Labels, "Labels should match")
}
