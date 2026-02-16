package suite_test

import (
	"errors"
	"testing"

	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

// TestPrerequisiteValidationDemo demonstrates prerequisite validation behavior
// This test is for documentation purposes and shows how tests fail early when prerequisites are not met
func TestPrerequisiteValidationDemo(t *testing.T) {
	t.Run("demonstrates early failure when Istio not installed", func(t *testing.T) {
		// Setup: Configure cluster without Istio
		suite.ClusterPrepConfig = &kubeprep.Config{
			InstallIstio:       false,
			EnableExperimental: false,
		}

		// This simulates what happens when a test with istio label runs
		// In real scenario, this would be called from RegisterTestCase
		checker := suite.NewPrerequisiteChecker(suite.ClusterPrepConfig)

		err := checker.ValidateLabels([]string{suite.LabelIstio})
		if err == nil {
			t.Fatal("Expected prerequisite validation to fail when Istio is not installed")
		}

		// Verify error message is helpful
		prereqErr := &suite.PrerequisiteError{}
		if errors.As(err, &prereqErr) {
			t.Logf("Got expected error: %s", prereqErr.Error())
			t.Logf("Suggestion: %s", prereqErr.Suggestion)
		} else {
			t.Fatalf("Expected PrerequisiteError, got: %T", err)
		}
	})

	t.Run("demonstrates early failure when experimental not enabled", func(t *testing.T) {
		// Setup: Configure cluster without experimental features
		suite.ClusterPrepConfig = &kubeprep.Config{
			InstallIstio:       false,
			EnableExperimental: false,
		}

		checker := suite.NewPrerequisiteChecker(suite.ClusterPrepConfig)

		err := checker.ValidateLabels([]string{suite.LabelExperimental})
		if err == nil {
			t.Fatal("Expected prerequisite validation to fail when experimental features are not enabled")
		}

		// Verify error message is helpful
		prereqErr := &suite.PrerequisiteError{}
		if errors.As(err, &prereqErr) {
			t.Logf("Got expected error: %s", prereqErr.Error())
			t.Logf("Suggestion: %s", prereqErr.Suggestion)
		} else {
			t.Fatalf("Expected PrerequisiteError, got: %T", err)
		}
	})

	t.Run("demonstrates success when prerequisites are met", func(t *testing.T) {
		// Setup: Configure cluster with all features
		suite.ClusterPrepConfig = &kubeprep.Config{
			InstallIstio:       true,
			EnableExperimental: true,
		}

		checker := suite.NewPrerequisiteChecker(suite.ClusterPrepConfig)

		// Test with istio label
		if err := checker.ValidateLabels([]string{suite.LabelIstio}); err != nil {
			t.Fatalf("Expected validation to pass with Istio installed: %v", err)
		}

		// Test with experimental label
		if err := checker.ValidateLabels([]string{suite.LabelExperimental}); err != nil {
			t.Fatalf("Expected validation to pass with experimental enabled: %v", err)
		}

		// Test with mixed labels
		if err := checker.ValidateLabels([]string{suite.LabelIstio, suite.LabelExperimental, suite.LabelLogAgent}); err != nil {
			t.Fatalf("Expected validation to pass with all prerequisites met: %v", err)
		}
	})

	t.Run("demonstrates filtering-only labels pass without prerequisites", func(t *testing.T) {
		// Setup: Configure cluster without any special features
		suite.ClusterPrepConfig = &kubeprep.Config{
			InstallIstio:       false,
			EnableExperimental: false,
		}

		checker := suite.NewPrerequisiteChecker(suite.ClusterPrepConfig)

		// These labels are for filtering only and don't require special prerequisites
		filteringLabels := []string{
			suite.LabelLogAgent,
			suite.LabelLogGateway,
			suite.LabelMetricAgentSetA,
			suite.LabelTraces,
			suite.LabelOAuth2,
			suite.LabelMTLS,
			suite.LabelGardener,
		}

		if err := checker.ValidateLabels(filteringLabels); err != nil {
			t.Fatalf("Expected validation to pass for filtering-only labels: %v", err)
		}
	})
}
