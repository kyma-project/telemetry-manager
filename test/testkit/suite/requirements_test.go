package suite

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
)

func TestInferRequirementsFromLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   []string
		expected kubeprep.Config
	}{
		{
			name:   "no labels - defaults (FIPS enabled)",
			labels: []string{},
			expected: kubeprep.Config{
				InstallIstio:          false,
				OperateInFIPSMode:     true, // FIPS enabled by default
				EnableExperimental:    false,
				SkipManagerDeployment: false,
				SkipPrerequisites:     false,
			},
		},
		{
			name:   "istio label",
			labels: []string{LabelIstio},
			expected: kubeprep.Config{
				InstallIstio:          true,
				OperateInFIPSMode:     true, // FIPS enabled by default
				EnableExperimental:    false,
				SkipManagerDeployment: false,
				SkipPrerequisites:     false,
			},
		},
		{
			name:   "experimental label",
			labels: []string{LabelExperimental},
			expected: kubeprep.Config{
				InstallIstio:          false,
				OperateInFIPSMode:     true, // FIPS enabled by default
				EnableExperimental:    true,
				SkipManagerDeployment: false,
				SkipPrerequisites:     false,
			},
		},
		{
			name:   "no-fips label disables FIPS",
			labels: []string{LabelNoFIPS},
			expected: kubeprep.Config{
				InstallIstio:          false,
				OperateInFIPSMode:     false, // Disabled by no-fips label
				EnableExperimental:    false,
				SkipManagerDeployment: false,
				SkipPrerequisites:     false,
			},
		},
		{
			name:   "istio and experimental",
			labels: []string{LabelIstio, LabelExperimental},
			expected: kubeprep.Config{
				InstallIstio:          true,
				OperateInFIPSMode:     true, // FIPS enabled by default
				EnableExperimental:    true,
				SkipManagerDeployment: false,
				SkipPrerequisites:     false,
			},
		},
		{
			name:   "istio, experimental and no-fips",
			labels: []string{LabelIstio, LabelExperimental, LabelNoFIPS},
			expected: kubeprep.Config{
				InstallIstio:          true,
				OperateInFIPSMode:     false, // Disabled by no-fips label
				EnableExperimental:    true,
				SkipManagerDeployment: false,
				SkipPrerequisites:     false,
			},
		},
		{
			name:   "unknown labels ignored",
			labels: []string{"unknown-label", "another-unknown"},
			expected: kubeprep.Config{
				InstallIstio:          false,
				OperateInFIPSMode:     true, // FIPS enabled by default
				EnableExperimental:    false,
				SkipManagerDeployment: false,
				SkipPrerequisites:     false,
			},
		},
		{
			name:   "mixed known and unknown labels",
			labels: []string{"unknown", LabelIstio, "another-unknown", LabelExperimental},
			expected: kubeprep.Config{
				InstallIstio:          true,
				OperateInFIPSMode:     true, // FIPS enabled by default
				EnableExperimental:    true,
				SkipManagerDeployment: false,
				SkipPrerequisites:     false,
			},
		},
		{
			name:   "other labels don't affect config",
			labels: []string{LabelLogAgent, LabelFluentBit},
			expected: kubeprep.Config{
				InstallIstio:          false,
				OperateInFIPSMode:     true, // FIPS enabled by default
				EnableExperimental:    false,
				SkipManagerDeployment: false,
				SkipPrerequisites:     false,
			},
		},
		{
			name:   "upgrade label uses default chart when env var not set",
			labels: []string{LabelUpgrade},
			expected: kubeprep.Config{
				InstallIstio:          false,
				OperateInFIPSMode:     true, // FIPS enabled by default
				EnableExperimental:    false,
				SkipManagerDeployment: false,
				SkipPrerequisites:     false,
				IsUpgradeTest:         true,
				UpgradeFromChart:      DefaultUpgradeFromChartURL, // Uses default when env var not set
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := InferRequirementsFromLabels(tt.labels)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestInferRequirementsFromLabels_UpgradeWithEnvVar(t *testing.T) {
	// Test upgrade label with UPGRADE_FROM_CHART env var set (overrides default)
	customChartURL := "https://github.com/kyma-project/telemetry-manager/releases/download/1.50.0/telemetry-manager-1.50.0.tgz"

	// Set environment variable
	t.Setenv("UPGRADE_FROM_CHART", customChartURL)

	actual := InferRequirementsFromLabels([]string{LabelUpgrade})

	expected := kubeprep.Config{
		InstallIstio:          false,
		OperateInFIPSMode:     true, // FIPS enabled by default
		EnableExperimental:    false,
		SkipManagerDeployment: false,
		SkipPrerequisites:     false,
		IsUpgradeTest:         true,
		UpgradeFromChart:      customChartURL, // Uses env var override
	}

	require.Equal(t, expected, actual)
}

func TestInferRequirementsFromLabels_UpgradeWithExperimental(t *testing.T) {
	// Test upgrade with experimental label - experimental should be enabled
	// Uses default chart URL since env var is not set

	actual := InferRequirementsFromLabels([]string{LabelUpgrade, LabelExperimental})

	expected := kubeprep.Config{
		InstallIstio:          false,
		OperateInFIPSMode:     true, // FIPS enabled by default
		EnableExperimental:    true, // From LabelExperimental
		SkipManagerDeployment: false,
		SkipPrerequisites:     false,
		IsUpgradeTest:         true,
		UpgradeFromChart:      DefaultUpgradeFromChartURL,
	}

	require.Equal(t, expected, actual)
}

func TestDefaultUpgradeFromChartURL(t *testing.T) {
	// Verify the default URL is correctly constructed
	expectedURL := "https://github.com/kyma-project/telemetry-manager/releases/download/1.57.2/telemetry-manager-1.57.2.tgz"
	require.Equal(t, expectedURL, DefaultUpgradeFromChartURL)
	require.Equal(t, "1.57.2", DefaultUpgradeFromVersion)
}
