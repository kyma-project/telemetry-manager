package suite

import (
	"os"

	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
)

const (
	// DefaultUpgradeFromVersion is the default version to upgrade from when
	// UPGRADE_FROM_CHART is not set. This allows running upgrade tests locally
	// without setting environment variables.
	DefaultUpgradeFromVersion = "1.57.2"

	// DefaultUpgradeFromChartURL is the helm chart URL for the default upgrade version.
	DefaultUpgradeFromChartURL = "https://github.com/kyma-project/telemetry-manager/releases/download/" +
		DefaultUpgradeFromVersion + "/telemetry-manager-" + DefaultUpgradeFromVersion + ".tgz"
)

// InferRequirementsFromLabels derives cluster configuration from test labels.
// This function maps test labels to cluster requirements, enabling dynamic
// cluster reconfiguration based on what each test needs.
//
// Label Mappings:
//   - LabelIstio: Requires Istio service mesh installation
//   - LabelExperimental: Requires experimental features enabled
//   - LabelNoFIPS: Disables FIPS mode (FIPS is enabled by default)
//   - LabelUpgrade: Upgrade test - deploys from UPGRADE_FROM_CHART (or default version)
//
// The returned Config contains all requirements derived from the labels.
// Fields not influenced by labels (like ManagerImage) are left at their
// default values and should be populated by the caller.
func InferRequirementsFromLabels(labels []string) kubeprep.Config {
	cfg := kubeprep.Config{
		// Defaults - most common case for tests
		InstallIstio:          false,
		OperateInFIPSMode:     true, // FIPS enabled by default
		EnableExperimental:    false,
		SkipManagerDeployment: false,
		SkipPrerequisites:     false,
	}

	// Scan labels and enable features as needed
	for _, label := range labels {
		switch label {
		case LabelIstio:
			cfg.InstallIstio = true
		case LabelExperimental:
			cfg.EnableExperimental = true
		case LabelNoFIPS:
			cfg.OperateInFIPSMode = false
		case LabelUpgrade:
			// For upgrade tests, use UPGRADE_FROM_CHART if set, otherwise use default
			cfg.UpgradeFromChart = os.Getenv("UPGRADE_FROM_CHART")
			if cfg.UpgradeFromChart == "" {
				cfg.UpgradeFromChart = DefaultUpgradeFromChartURL
			}

			cfg.IsUpgradeTest = true
		}
	}

	return cfg
}
