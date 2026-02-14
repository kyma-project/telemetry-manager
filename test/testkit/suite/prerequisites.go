package suite

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
)

// PrerequisiteChecker validates test prerequisites based on cluster configuration
type PrerequisiteChecker struct {
	config *kubeprep.Config
}

// PrerequisiteError represents a failed prerequisite check
type PrerequisiteError struct {
	Label       string
	Requirement string
	Suggestion  string
}

func (e *PrerequisiteError) Error() string {
	return fmt.Sprintf("prerequisite check failed for label '%s': %s", e.Label, e.Requirement)
}

// NewPrerequisiteChecker creates a new prerequisite checker
func NewPrerequisiteChecker(config *kubeprep.Config) *PrerequisiteChecker {
	return &PrerequisiteChecker{config: config}
}

// ValidateLabels checks if the cluster configuration satisfies test prerequisites
func (pc *PrerequisiteChecker) ValidateLabels(labels []string) error {
	for _, label := range labels {
		switch label {
		case LabelIstio:
			if !pc.config.InstallIstio {
				return &PrerequisiteError{
					Label:       label,
					Requirement: "Istio must be installed in the cluster",
					Suggestion:  "Set INSTALL_ISTIO=true environment variable or run: make setup-e2e-istio",
				}
			}

		case LabelExperimental:
			if !pc.config.EnableExperimental {
				return &PrerequisiteError{
					Label:       label,
					Requirement: "Experimental features must be enabled",
					Suggestion:  "Set ENABLE_EXPERIMENTAL=true environment variable or run: make setup-e2e-experimental",
				}
			}

		// Other labels don't require special prerequisites
		// (logs, metrics, traces, oauth2, mtls, gardener, fips, etc. are filtering labels only)
		default:
			continue
		}
	}
	return nil
}
