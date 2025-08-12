package common

import (
	"testing"

	"github.com/stretchr/testify/require"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
)

func TestK8sAttributesProcessorConfig(t *testing.T) {
	require := require.New(t)

	expectedPodAssociations := []PodAssociations{
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
	expectedExtractLabels := []ExtractLabel{
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
		{
			From:     "pod",
			KeyRegex: "(app.kubernetes.io/name.*)",
			TagName:  "k8s.pod.label.$0",
		},
		{
			From:     "pod",
			KeyRegex: "(^app$)",
			TagName:  "k8s.pod.label.$0",
		},
	}

	config := K8sAttributesProcessorConfig(&operatorv1alpha1.EnrichmentSpec{
		ExtractPodLabels: []operatorv1alpha1.PodLabel{
			{Key: "", KeyPrefix: "app.kubernetes.io/name"},
			{Key: "app", KeyPrefix: ""},
		},
	})

	require.Equal("serviceAccount", config.AuthType)
	require.Equal(false, config.Passthrough)
	require.Equal(expectedPodAssociations, config.PodAssociation, "PodAssociation should match")

	require.ElementsMatch(expectedK8sAttributes, config.Extract.Metadata, "Metadata should match")
	require.ElementsMatch(expectedExtractLabels, config.Extract.Labels, "Labels should match")
}

func TestBuildPodLabelEnrichments(t *testing.T) {
	tests := []struct {
		name     string
		presets  *operatorv1alpha1.EnrichmentSpec
		expected []ExtractLabel
	}{
		{
			name: "Enrichments disabled",
			presets: &operatorv1alpha1.EnrichmentSpec{
				ExtractPodLabels: []operatorv1alpha1.PodLabel{},
			},
			expected: []ExtractLabel{},
		},
		{
			name: "Enrichments enabled with key",
			presets: &operatorv1alpha1.EnrichmentSpec{
				ExtractPodLabels: []operatorv1alpha1.PodLabel{
					{Key: "app"},
				},
			},
			expected: []ExtractLabel{
				{
					From:     "pod",
					KeyRegex: "(^app$)",
					TagName:  "k8s.pod.label.$0",
				},
			},
		},
		{
			name: "Enrichments enabled with key prefix",
			presets: &operatorv1alpha1.EnrichmentSpec{
				ExtractPodLabels: []operatorv1alpha1.PodLabel{
					{KeyPrefix: "app.kubernetes.io"},
				},
			},
			expected: []ExtractLabel{
				{
					From:     "pod",
					KeyRegex: "(app.kubernetes.io.*)",
					TagName:  "k8s.pod.label.$0",
				},
			},
		},
		{
			name: "Enrichments enabled with multiple labels",
			presets: &operatorv1alpha1.EnrichmentSpec{
				ExtractPodLabels: []operatorv1alpha1.PodLabel{
					{Key: "app"},
					{KeyPrefix: "app.kubernetes.io"},
				},
			},
			expected: []ExtractLabel{
				{
					From:     "pod",
					KeyRegex: "(^app$)",
					TagName:  "k8s.pod.label.$0",
				},
				{
					From:     "pod",
					KeyRegex: "(app.kubernetes.io.*)",
					TagName:  "k8s.pod.label.$0",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			result := buildExtractPodLabels(tt.presets)
			require.ElementsMatch(tt.expected, result)
		})
	}
}
