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

	config := K8sAttributesProcessorConfig(EnrichmentOpts{
		Enabled: true,
		PodLabels: []PodLabel{
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
		presets  EnrichmentOpts
		expected []config.ExtractLabel
	}{
		{
			name: "Enrichments disabled",
			presets: EnrichmentOpts{
				Enabled:   false,
				PodLabels: []PodLabel{},
			},
			expected: []config.ExtractLabel{},
		},
		{
			name: "Enrichments enabled with key",
			presets: EnrichmentOpts{
				Enabled: true,
				PodLabels: []PodLabel{
					{Key: "app"},
				},
			},
			expected: []config.ExtractLabel{
				{
					From:     "pod",
					KeyRegex: "(^app$)",
					TagName:  "k8s.pod.label.$0",
				},
			},
		},
		{
			name: "Enrichments enabled with key prefix",
			presets: EnrichmentOpts{
				Enabled: true,
				PodLabels: []PodLabel{
					{KeyPrefix: "app.kubernetes.io"},
				},
			},
			expected: []config.ExtractLabel{
				{
					From:     "pod",
					KeyRegex: "(app.kubernetes.io.*)",
					TagName:  "k8s.pod.label.$0",
				},
			},
		},
		{
			name: "Enrichments enabled with multiple labels",
			presets: EnrichmentOpts{
				Enabled: true,
				PodLabels: []PodLabel{
					{Key: "app"},
					{KeyPrefix: "app.kubernetes.io"},
				},
			},
			expected: []config.ExtractLabel{
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
			result := buildPodLabelPresets(tt.presets)
			require.ElementsMatch(tt.expected, result)
		})
	}
}
