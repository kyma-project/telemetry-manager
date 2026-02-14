package kubeprep

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

// Config contains cluster preparation configuration
type Config struct {
	ManagerImage            string // Required: telemetry manager container image
	LocalImage              bool   // Image is local (for k3d import and pull policy)
	InstallIstio            bool   // Install Istio before tests
	OperateInFIPSMode       bool   // Deploy manager in FIPS mode
	EnableExperimental      bool   // Enable experimental CRDs
	CustomLabelsAnnotations bool   // Enable custom labels/annotations
	SkipManagerDeployment   bool   // For upgrade tests
	SkipPrerequisites       bool   // For custom test setups
	NeedsReinstall          bool   // Manager state is unknown, needs reinstallation
	UpgradeFromChart        string // Helm chart URL for old version (upgrade tests only)
	IsUpgradeTest           bool   // Test is an upgrade test (affects initial deployment)
}

// ConfigFromEnv loads cluster preparation configuration from environment variables
func ConfigFromEnv() (*Config, error) {
	managerImage := os.Getenv("MANAGER_IMAGE")
	if managerImage == "" {
		return nil, fmt.Errorf("MANAGER_IMAGE environment variable is required")
	}

	// Auto-detect if image is local
	localImage := isLocalImage(managerImage)

	// Allow explicit override via LOCAL_IMAGE env var
	if localImageEnv := os.Getenv("LOCAL_IMAGE"); localImageEnv != "" {
		localImage = parseBool(localImageEnv, localImage)
	}

	cfg := &Config{
		ManagerImage:            managerImage,
		LocalImage:              localImage,
		InstallIstio:            parseBool(os.Getenv("INSTALL_ISTIO"), false),
		OperateInFIPSMode:       parseBool(os.Getenv("OPERATE_IN_FIPS_MODE"), true),
		EnableExperimental:      parseBool(os.Getenv("ENABLE_EXPERIMENTAL"), false),
		CustomLabelsAnnotations: parseBool(os.Getenv("CUSTOM_LABELS_ANNOTATIONS"), false),
		SkipManagerDeployment:   parseBool(os.Getenv("SKIP_MANAGER_DEPLOYMENT"), false),
		SkipPrerequisites:       parseBool(os.Getenv("SKIP_PREREQUISITES"), false),
		UpgradeFromChart:        os.Getenv("UPGRADE_FROM_CHART"),
	}

	log.Printf("Detected image type: local=%t for image=%s", cfg.LocalImage, cfg.ManagerImage)

	return cfg, nil
}

// ConfigWithDefaults returns a configuration with sane defaults for IDE/local testing
// when MANAGER_IMAGE is not set. Assumes: local image, no Istio, standard telemetry installation.
// Environment variables can still override individual settings.
func ConfigWithDefaults() *Config {
	// Default to a local image - user should have telemetry-manager:latest pre-built
	managerImage := os.Getenv("MANAGER_IMAGE")
	if managerImage == "" {
		managerImage = "telemetry-manager:latest"
	}

	// Auto-detect if image is local
	localImage := isLocalImage(managerImage)

	// Allow explicit override via LOCAL_IMAGE env var
	if localImageEnv := os.Getenv("LOCAL_IMAGE"); localImageEnv != "" {
		localImage = parseBool(localImageEnv, localImage)
	}

	cfg := &Config{
		ManagerImage:            managerImage,
		LocalImage:              localImage,
		InstallIstio:            parseBool(os.Getenv("INSTALL_ISTIO"), false),        // No Istio by default
		OperateInFIPSMode:       parseBool(os.Getenv("OPERATE_IN_FIPS_MODE"), false), // No FIPS by default
		EnableExperimental:      parseBool(os.Getenv("ENABLE_EXPERIMENTAL"), false),
		CustomLabelsAnnotations: parseBool(os.Getenv("CUSTOM_LABELS_ANNOTATIONS"), false),
		SkipManagerDeployment:   parseBool(os.Getenv("SKIP_MANAGER_DEPLOYMENT"), false),
		SkipPrerequisites:       parseBool(os.Getenv("SKIP_PREREQUISITES"), false),
		UpgradeFromChart:        os.Getenv("UPGRADE_FROM_CHART"),
	}

	log.Printf("Using default configuration: image=%s, local=%t, istio=%t, fips=%t",
		cfg.ManagerImage, cfg.LocalImage, cfg.InstallIstio, cfg.OperateInFIPSMode)

	return cfg
}

// parseBool parses a string as a boolean with a default value
func parseBool(s string, defaultValue bool) bool {
	if s == "" {
		return defaultValue
	}
	val, err := strconv.ParseBool(s)
	if err != nil {
		return defaultValue
	}
	return val
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
