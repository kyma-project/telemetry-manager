package kubeprep

import (
	"strings"
)

// Config contains cluster preparation configuration
type Config struct {
	ManagerImage          string   // Required: telemetry manager container image
	LocalImage            bool     // Image is local (for k3d import and pull policy)
	InstallIstio          bool     // Install Istio before tests
	OperateInFIPSMode     bool     // Deploy manager in FIPS mode
	EnableExperimental    bool     // Enable experimental CRDs
	HelmValues            []string // Custom helm --set values (e.g., "additionalMetadata.labels.foo=bar")
	SkipManagerDeployment bool     // For upgrade tests
	SkipPrerequisites     bool     // For custom test setups
	NeedsReinstall        bool     // Manager state is unknown, needs reinstallation
	UpgradeFromChart      string   // Helm chart URL for old version (upgrade tests only)
	IsUpgradeTest         bool     // Test is an upgrade test (affects initial deployment)
}

// isLocalImage detects if an image is local (not from a remote registry)
func isLocalImage(image string) bool {
	// Remote registries
	remoteRegistries := []string{
		"docker.pkg.dev",
		"gcr.io",
		"ghcr.io",
		"docker.io",
		"quay.io",
	}

	for _, registry := range remoteRegistries {
		if strings.Contains(image, registry) {
			return false
		}
	}

	return true
}

// IsLocalImage is a public wrapper for isLocalImage
func IsLocalImage(image string) bool {
	return isLocalImage(image)
}
