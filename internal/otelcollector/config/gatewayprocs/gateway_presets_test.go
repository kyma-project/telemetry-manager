package gatewayprocs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

func TestBuildPodLabelPresets(t *testing.T) {
	tests := []struct {
		name     string
		presets  Presets
		expected []config.ExtractLabel
	}{
		{
			name: "Presets disabled",
			presets: Presets{
				Enabled:   false,
				PodLabels: []PodLabel{},
			},
			expected: []config.ExtractLabel{},
		},
		{
			name: "Presets enabled with key",
			presets: Presets{
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
			name: "Presets enabled with key prefix",
			presets: Presets{
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
			name: "Presets enabled with multiple labels",
			presets: Presets{
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
