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
				InstallIstio:            false,
				OperateInFIPSMode:       false,
				EnableExperimental:      false,
				CustomLabelsAnnotations: false,
				SkipManagerDeployment:   false,
				SkipPrerequisites:       false,
			},
			desired: Config{
				InstallIstio:            false,
				OperateInFIPSMode:       false,
				EnableExperimental:      false,
				CustomLabelsAnnotations: false,
				SkipManagerDeployment:   false,
				SkipPrerequisites:       false,
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
			name: "custom labels annotations change",
			current: Config{
				CustomLabelsAnnotations: false,
			},
			desired: Config{
				CustomLabelsAnnotations: true,
			},
			expected: ConfigDiff{
				NeedsIstioChange:         false,
				NeedsManagerRedeploy:     true,
				NeedsPrerequisitesUpdate: true,
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
			name: "multiple changes - experimental and custom labels",
			current: Config{
				EnableExperimental:      false,
				CustomLabelsAnnotations: false,
			},
			desired: Config{
				EnableExperimental:      true,
				CustomLabelsAnnotations: true,
			},
			expected: ConfigDiff{
				NeedsIstioChange:         false,
				NeedsManagerRedeploy:     true,
				NeedsPrerequisitesUpdate: true,
			},
		},
		{
			name: "all changes at once",
			current: Config{
				InstallIstio:            false,
				OperateInFIPSMode:       false,
				EnableExperimental:      false,
				CustomLabelsAnnotations: false,
				SkipManagerDeployment:   false,
				SkipPrerequisites:       false,
			},
			desired: Config{
				InstallIstio:            true,
				OperateInFIPSMode:       true,
				EnableExperimental:      true,
				CustomLabelsAnnotations: true,
				SkipManagerDeployment:   false,
				SkipPrerequisites:       false,
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
