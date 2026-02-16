package suite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
)

func TestPrerequisiteChecker_ValidateLabels_IstioLabel(t *testing.T) {
	t.Run("should pass when Istio is installed", func(t *testing.T) {
		config := &kubeprep.Config{
			InstallIstio: true,
		}
		checker := NewPrerequisiteChecker(config)

		err := checker.ValidateLabels([]string{LabelIstio})

		assert.NoError(t, err)
	})

	t.Run("should fail when Istio is not installed", func(t *testing.T) {
		config := &kubeprep.Config{
			InstallIstio: false,
		}
		checker := NewPrerequisiteChecker(config)

		err := checker.ValidateLabels([]string{LabelIstio})

		require.Error(t, err)

		var prereqErr *PrerequisiteError
		require.ErrorAs(t, err, &prereqErr)
		assert.Equal(t, LabelIstio, prereqErr.Label)
		assert.Contains(t, prereqErr.Requirement, "Istio must be installed")
		assert.Contains(t, prereqErr.Suggestion, "make setup-e2e-istio")
	})
}

func TestPrerequisiteChecker_ValidateLabels_ExperimentalLabel(t *testing.T) {
	t.Run("should pass when experimental features are enabled", func(t *testing.T) {
		config := &kubeprep.Config{
			EnableExperimental: true,
		}
		checker := NewPrerequisiteChecker(config)

		err := checker.ValidateLabels([]string{LabelExperimental})

		assert.NoError(t, err)
	})

	t.Run("should fail when experimental features are not enabled", func(t *testing.T) {
		config := &kubeprep.Config{
			EnableExperimental: false,
		}
		checker := NewPrerequisiteChecker(config)

		err := checker.ValidateLabels([]string{LabelExperimental})

		require.Error(t, err)

		var prereqErr *PrerequisiteError
		require.ErrorAs(t, err, &prereqErr)
		assert.Equal(t, LabelExperimental, prereqErr.Label)
		assert.Contains(t, prereqErr.Requirement, "Experimental features must be enabled")
		assert.Contains(t, prereqErr.Suggestion, "make setup-e2e-experimental")
	})
}

func TestPrerequisiteChecker_ValidateLabels_MultipleLabels(t *testing.T) {
	t.Run("should pass when all prerequisites are met", func(t *testing.T) {
		config := &kubeprep.Config{
			InstallIstio:       true,
			EnableExperimental: true,
		}
		checker := NewPrerequisiteChecker(config)

		err := checker.ValidateLabels([]string{LabelIstio, LabelExperimental})

		assert.NoError(t, err)
	})

	t.Run("should fail on first unmet prerequisite", func(t *testing.T) {
		config := &kubeprep.Config{
			InstallIstio:       false,
			EnableExperimental: true,
		}
		checker := NewPrerequisiteChecker(config)

		err := checker.ValidateLabels([]string{LabelIstio, LabelExperimental})

		require.Error(t, err)

		var prereqErr *PrerequisiteError
		require.ErrorAs(t, err, &prereqErr)
		assert.Equal(t, LabelIstio, prereqErr.Label)
	})

	t.Run("should pass when prerequisites are met with filtering labels mixed in", func(t *testing.T) {
		config := &kubeprep.Config{
			InstallIstio:       true,
			EnableExperimental: false,
		}
		checker := NewPrerequisiteChecker(config)

		// Mix prerequisite labels with filtering-only labels
		err := checker.ValidateLabels([]string{LabelIstio, LabelLogAgent, LabelMetricAgentSetA, LabelOAuth2})

		assert.NoError(t, err)
	})
}

func TestPrerequisiteChecker_ValidateLabels_FilteringLabels(t *testing.T) {
	t.Run("should pass for filtering-only labels without special prerequisites", func(t *testing.T) {
		config := &kubeprep.Config{
			InstallIstio:       false,
			EnableExperimental: false,
		}
		checker := NewPrerequisiteChecker(config)

		filteringLabels := []string{
			LabelLogAgent,
			LabelLogGateway,
			LabelMetricAgentSetA,
			LabelMetricGatewaySetA,
			LabelTraces,
			LabelOAuth2,
			LabelMTLS,
			LabelGardener,
			LabelCustomLabelAnnotation,
			LabelMisc,
			LabelTelemetry,
			// Any other filtering labels
		}

		err := checker.ValidateLabels(filteringLabels)

		assert.NoError(t, err)
	})
}

func TestPrerequisiteChecker_ValidateLabels_EdgeCases(t *testing.T) {
	t.Run("should pass for empty label list", func(t *testing.T) {
		config := &kubeprep.Config{}
		checker := NewPrerequisiteChecker(config)

		err := checker.ValidateLabels([]string{})

		assert.NoError(t, err)
	})

	t.Run("should pass for unknown labels", func(t *testing.T) {
		config := &kubeprep.Config{}
		checker := NewPrerequisiteChecker(config)

		err := checker.ValidateLabels([]string{"unknown-label", "another-unknown"})

		assert.NoError(t, err)
	})

	t.Run("should handle duplicate labels", func(t *testing.T) {
		config := &kubeprep.Config{
			InstallIstio: true,
		}
		checker := NewPrerequisiteChecker(config)

		err := checker.ValidateLabels([]string{LabelIstio, LabelIstio, LabelIstio})

		assert.NoError(t, err)
	})

	t.Run("should fail on duplicate unmet prerequisites", func(t *testing.T) {
		config := &kubeprep.Config{
			InstallIstio: false,
		}
		checker := NewPrerequisiteChecker(config)

		err := checker.ValidateLabels([]string{LabelIstio, LabelIstio})

		require.Error(t, err)

		var prereqErr *PrerequisiteError
		require.ErrorAs(t, err, &prereqErr)
		assert.Equal(t, LabelIstio, prereqErr.Label)
	})
}

func TestPrerequisiteError_Error(t *testing.T) {
	t.Run("should format error message correctly", func(t *testing.T) {
		err := &PrerequisiteError{
			Label:       "test-label",
			Requirement: "Test requirement",
			Suggestion:  "Test suggestion",
		}

		errMsg := err.Error()

		assert.Contains(t, errMsg, "test-label")
		assert.Contains(t, errMsg, "Test requirement")
		assert.Contains(t, errMsg, "prerequisite check failed")
	})
}

func TestNewPrerequisiteChecker(t *testing.T) {
	t.Run("should create checker with config", func(t *testing.T) {
		config := &kubeprep.Config{
			InstallIstio:       true,
			EnableExperimental: false,
		}

		checker := NewPrerequisiteChecker(config)

		require.NotNil(t, checker)
		assert.Equal(t, config, checker.config)
	})
}
