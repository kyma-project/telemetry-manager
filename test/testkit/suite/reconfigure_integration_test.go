package suite

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
)

// TestConfigsEqual verifies the config comparison logic
func TestConfigsEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        kubeprep.Config
		b        kubeprep.Config
		expected bool
	}{
		{
			name: "identical configs are equal",
			a: kubeprep.Config{
				ManagerImage:          "manager:v1",
				LocalImage:            true,
				InstallIstio:          false,
				OperateInFIPSMode:     false,
				EnableExperimental:    false,
				SkipManagerDeployment: false,
				SkipPrerequisites:     false,
			},
			b: kubeprep.Config{
				ManagerImage:          "manager:v1",
				LocalImage:            true,
				InstallIstio:          false,
				OperateInFIPSMode:     false,
				EnableExperimental:    false,
				SkipManagerDeployment: false,
				SkipPrerequisites:     false,
			},
			expected: true,
		},
		{
			name: "manager image difference is ignored",
			a: kubeprep.Config{
				ManagerImage: "manager:v1",
				LocalImage:   true,
			},
			b: kubeprep.Config{
				ManagerImage: "manager:v2",
				LocalImage:   false,
			},
			expected: true,
		},
		{
			name: "istio difference is detected",
			a: kubeprep.Config{
				InstallIstio: false,
			},
			b: kubeprep.Config{
				InstallIstio: true,
			},
			expected: false,
		},
		{
			name: "fips difference is detected",
			a: kubeprep.Config{
				OperateInFIPSMode: false,
			},
			b: kubeprep.Config{
				OperateInFIPSMode: true,
			},
			expected: false,
		},
		{
			name: "experimental difference is detected",
			a: kubeprep.Config{
				EnableExperimental: false,
			},
			b: kubeprep.Config{
				EnableExperimental: true,
			},
			expected: false,
		},
		{
			name: "helm values difference is detected",
			a: kubeprep.Config{
				HelmValues: nil,
			},
			b: kubeprep.Config{
				HelmValues: []string{"foo=bar"},
			},
			expected: false,
		},
		{
			name: "helm values same order-independent",
			a: kubeprep.Config{
				HelmValues: []string{"foo=bar", "baz=qux"},
			},
			b: kubeprep.Config{
				HelmValues: []string{"baz=qux", "foo=bar"},
			},
			expected: true,
		},
		{
			name: "skip manager deployment difference is detected",
			a: kubeprep.Config{
				SkipManagerDeployment: false,
			},
			b: kubeprep.Config{
				SkipManagerDeployment: true,
			},
			expected: false,
		},
		{
			name: "skip prerequisites difference is detected",
			a: kubeprep.Config{
				SkipPrerequisites: false,
			},
			b: kubeprep.Config{
				SkipPrerequisites: true,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := configsEqual(tt.a, tt.b)
			require.Equal(t, tt.expected, actual)
		})
	}
}

// TestEnsureClusterState_NoReconfiguration demonstrates that no reconfiguration
// happens when the cluster is already in the desired state
func TestEnsureClusterState_NoReconfiguration(t *testing.T) {
	// Save original state
	origConfig := ClusterPrepConfig
	origState := CurrentClusterState

	defer func() {
		ClusterPrepConfig = origConfig
		CurrentClusterState = origState
	}()

	// Set up initial state
	initialConfig := &kubeprep.Config{
		ManagerImage:          "manager:v1",
		LocalImage:            true,
		InstallIstio:          false,
		OperateInFIPSMode:     false,
		EnableExperimental:    false,
		SkipManagerDeployment: false,
		SkipPrerequisites:     false,
	}

	ClusterPrepConfig = initialConfig
	CurrentClusterState = &kubeprep.Config{
		ManagerImage:          "manager:v1",
		LocalImage:            true,
		InstallIstio:          false,
		OperateInFIPSMode:     false,
		EnableExperimental:    false,
		SkipManagerDeployment: false,
		SkipPrerequisites:     false,
	}

	// Infer requirements from labels that match current state
	// Use LabelNoFIPS to disable FIPS (matching the current state with OperateInFIPSMode: false)
	requiredConfig := InferRequirementsFromLabels([]string{LabelLogAgent, LabelNoFIPS})

	// This should not trigger any reconfiguration
	// We can't actually call ensureClusterState here because it requires a real K8s client
	// But we can verify that the configs are equal
	requiredConfig.ManagerImage = CurrentClusterState.ManagerImage
	requiredConfig.LocalImage = CurrentClusterState.LocalImage

	require.True(t, configsEqual(*CurrentClusterState, requiredConfig),
		"Configs should be equal, no reconfiguration needed")
}

// TestEnsureClusterState_RequiresReconfiguration demonstrates that reconfiguration
// is detected when the cluster state doesn't match requirements
func TestEnsureClusterState_RequiresReconfiguration(t *testing.T) {
	// Save original state
	origConfig := ClusterPrepConfig
	origState := CurrentClusterState

	defer func() {
		ClusterPrepConfig = origConfig
		CurrentClusterState = origState
	}()

	// Set up initial state (no Istio)
	initialConfig := &kubeprep.Config{
		ManagerImage:          "manager:v1",
		LocalImage:            true,
		InstallIstio:          false,
		OperateInFIPSMode:     false,
		EnableExperimental:    false,
		SkipManagerDeployment: false,
		SkipPrerequisites:     false,
	}

	ClusterPrepConfig = initialConfig
	CurrentClusterState = &kubeprep.Config{
		ManagerImage:          "manager:v1",
		LocalImage:            true,
		InstallIstio:          false,
		OperateInFIPSMode:     false,
		EnableExperimental:    false,
		SkipManagerDeployment: false,
		SkipPrerequisites:     false,
	}

	// Infer requirements from Istio label (requires Istio)
	requiredConfig := InferRequirementsFromLabels([]string{LabelIstio})
	requiredConfig.ManagerImage = CurrentClusterState.ManagerImage
	requiredConfig.LocalImage = CurrentClusterState.LocalImage

	require.False(t, configsEqual(*CurrentClusterState, requiredConfig),
		"Configs should differ, reconfiguration needed")
	require.True(t, requiredConfig.InstallIstio,
		"Required config should have Istio enabled")
	require.False(t, CurrentClusterState.InstallIstio,
		"Current state should have Istio disabled")
}
