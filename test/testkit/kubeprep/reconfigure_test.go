package kubeprep

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalculateDiff(t *testing.T) {
	tests := []struct {
		name     string
		current  Config
		desired  Config
		expected ConfigDiff
	}{
		{
			name: "no changes",
			current: Config{
				InstallIstio:          false,
				OperateInFIPSMode:     false,
				EnableExperimental:    false,
				SkipManagerDeployment: false,
				SkipPrerequisites:     false,
			},
			desired: Config{
				InstallIstio:          false,
				OperateInFIPSMode:     false,
				EnableExperimental:    false,
				SkipManagerDeployment: false,
				SkipPrerequisites:     false,
			},
			expected: ConfigDiff{
				NeedsIstioChange:         false,
				NeedsManagerRedeploy:     false,
				NeedsPrerequisitesUpdate: false,
			},
		},
		{
			name: "istio installation required",
			current: Config{
				InstallIstio: false,
			},
			desired: Config{
				InstallIstio: true,
			},
			expected: ConfigDiff{
				NeedsIstioChange:         true,
				NeedsManagerRedeploy:     false,
				NeedsPrerequisitesUpdate: false,
			},
		},
		{
			name: "istio uninstallation required",
			current: Config{
				InstallIstio: true,
			},
			desired: Config{
				InstallIstio: false,
			},
			expected: ConfigDiff{
				NeedsIstioChange:         true,
				NeedsManagerRedeploy:     false,
				NeedsPrerequisitesUpdate: false,
			},
		},
		{
			name: "fips mode change",
			current: Config{
				OperateInFIPSMode: false,
			},
			desired: Config{
				OperateInFIPSMode: true,
			},
			expected: ConfigDiff{
				NeedsIstioChange:         false,
				NeedsManagerRedeploy:     true,
				NeedsPrerequisitesUpdate: false,
			},
		},
		{
			name: "experimental change",
			current: Config{
				EnableExperimental: false,
			},
			desired: Config{
				EnableExperimental: true,
			},
			expected: ConfigDiff{
				NeedsIstioChange:         false,
				NeedsManagerRedeploy:     true,
				NeedsPrerequisitesUpdate: false,
			},
		},
		{
			name: "helm values change - triggers redeploy",
			current: Config{
				HelmValues: nil,
			},
			desired: Config{
				HelmValues: []string{"foo=bar"},
			},
			expected: ConfigDiff{
				NeedsIstioChange:         false,
				NeedsManagerRedeploy:     true,
				NeedsPrerequisitesUpdate: false,
			},
		},
		{
			name: "helm values same - no redeploy",
			current: Config{
				HelmValues: []string{"foo=bar", "baz=qux"},
			},
			desired: Config{
				HelmValues: []string{"baz=qux", "foo=bar"}, // Different order but same values
			},
			expected: ConfigDiff{
				NeedsIstioChange:         false,
				NeedsManagerRedeploy:     false,
				NeedsPrerequisitesUpdate: false,
			},
		},
		{
			name: "skip manager deployment change",
			current: Config{
				SkipManagerDeployment: false,
			},
			desired: Config{
				SkipManagerDeployment: true,
			},
			expected: ConfigDiff{
				NeedsIstioChange:         false,
				NeedsManagerRedeploy:     true,
				NeedsPrerequisitesUpdate: false,
			},
		},
		{
			name: "skip prerequisites change",
			current: Config{
				SkipPrerequisites: false,
			},
			desired: Config{
				SkipPrerequisites: true,
			},
			expected: ConfigDiff{
				NeedsIstioChange:         false,
				NeedsManagerRedeploy:     false,
				NeedsPrerequisitesUpdate: true,
			},
		},
		{
			name: "multiple changes - istio and fips",
			current: Config{
				InstallIstio:      false,
				OperateInFIPSMode: false,
			},
			desired: Config{
				InstallIstio:      true,
				OperateInFIPSMode: true,
			},
			expected: ConfigDiff{
				NeedsIstioChange:         true,
				NeedsManagerRedeploy:     true,
				NeedsPrerequisitesUpdate: false,
			},
		},
		{
			name: "multiple changes - experimental and helm values",
			current: Config{
				EnableExperimental: false,
				HelmValues:         nil,
			},
			desired: Config{
				EnableExperimental: true,
				HelmValues:         []string{"foo=bar"},
			},
			expected: ConfigDiff{
				NeedsIstioChange:         false,
				NeedsManagerRedeploy:     true,
				NeedsPrerequisitesUpdate: false,
			},
		},
		{
			name: "all changes at once",
			current: Config{
				InstallIstio:          false,
				OperateInFIPSMode:     false,
				EnableExperimental:    false,
				HelmValues:            nil,
				SkipManagerDeployment: false,
				SkipPrerequisites:     false,
			},
			desired: Config{
				InstallIstio:          true,
				OperateInFIPSMode:     true,
				EnableExperimental:    true,
				HelmValues:            []string{"foo=bar"},
				SkipManagerDeployment: false,
				SkipPrerequisites:     true,
			},
			expected: ConfigDiff{
				NeedsIstioChange:         true,
				NeedsManagerRedeploy:     true,
				NeedsPrerequisitesUpdate: true,
			},
		},
		{
			name: "image changes don't trigger reconfiguration",
			current: Config{
				ManagerImage: "manager:v1",
				LocalImage:   true,
			},
			desired: Config{
				ManagerImage: "manager:v2",
				LocalImage:   false,
			},
			expected: ConfigDiff{
				NeedsIstioChange:         false,
				NeedsManagerRedeploy:     false,
				NeedsPrerequisitesUpdate: false,
			},
		},
		{
			name: "needs reinstall triggers manager redeploy",
			current: Config{
				NeedsReinstall:     true, // Unknown state, needs reinstall
				EnableExperimental: false,
			},
			desired: Config{
				NeedsReinstall:     false,
				EnableExperimental: false,
			},
			expected: ConfigDiff{
				NeedsIstioChange:         false,
				NeedsManagerRedeploy:     true, // Triggered by NeedsReinstall
				NeedsPrerequisitesUpdate: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := calculateDiff(tt.current, tt.desired)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestHelmValuesEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected bool
	}{
		{
			name:     "both nil",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "both empty",
			a:        []string{},
			b:        []string{},
			expected: true,
		},
		{
			name:     "nil vs empty - equal",
			a:        nil,
			b:        []string{},
			expected: true,
		},
		{
			name:     "same values same order",
			a:        []string{"foo=bar", "baz=qux"},
			b:        []string{"foo=bar", "baz=qux"},
			expected: true,
		},
		{
			name:     "same values different order",
			a:        []string{"foo=bar", "baz=qux"},
			b:        []string{"baz=qux", "foo=bar"},
			expected: true,
		},
		{
			name:     "different lengths",
			a:        []string{"foo=bar"},
			b:        []string{"foo=bar", "baz=qux"},
			expected: false,
		},
		{
			name:     "different values",
			a:        []string{"foo=bar"},
			b:        []string{"foo=baz"},
			expected: false,
		},
		{
			name:     "one nil one with values",
			a:        nil,
			b:        []string{"foo=bar"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := helmValuesEqual(tt.a, tt.b)
			require.Equal(t, tt.expected, actual)
		})
	}
}
