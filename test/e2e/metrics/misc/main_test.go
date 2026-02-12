package misc

import (
	"log"
	"os"
	"testing"

	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestMain(m *testing.M) {
	const errorCode = 1

	// Load cluster configuration from environment or use defaults
	// If MANAGER_IMAGE is set, use full environment configuration
	// If not set, use sane defaults (local image, no Istio)
	var cfg *kubeprep.Config
	if managerImage := os.Getenv("MANAGER_IMAGE"); managerImage != "" {
		var err error
		cfg, err = kubeprep.ConfigFromEnv()
		if err != nil {
			log.Printf("Failed to load cluster config: %v", err)
			os.Exit(errorCode)
		}
		log.Println("Using cluster configuration from environment (MANAGER_IMAGE is set)")
	} else {
		// Running from IDE or local testing
		// Use sane defaults: local image, no Istio, proper telemetry installation
		cfg = kubeprep.ConfigWithDefaults()
		log.Println("Using default cluster configuration (MANAGER_IMAGE not set)")
		log.Println("Defaults: telemetry-manager:latest (local), no Istio, no FIPS")
	}

	suite.ClusterPrepConfig = cfg

	if err := suite.BeforeSuiteFunc(); err != nil {
		log.Printf("Setup failed: %v", err)
		// Cleanup before exiting
		if cfg != nil {
			log.Println("Cleaning up cluster after setup failure...")
			if cleanupErr := suite.AfterSuiteFunc(); cleanupErr != nil {
				log.Printf("Warning: cleanup failed: %v", cleanupErr)
			}
		}
		os.Exit(errorCode)
	}

	// Run tests
	exitCode := m.Run()

	// Always cleanup after tests complete
	if cfg != nil {
		log.Println("Cleaning up cluster after tests...")
		if err := suite.AfterSuiteFunc(); err != nil {
			log.Printf("Warning: cleanup failed: %v", err)
		}
	}

	// Only exit with non-zero code if tests failed
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
