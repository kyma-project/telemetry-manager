package agent

import (
	"log"
	"os"
	"testing"

	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestMain(m *testing.M) {
	// No explicit cluster configuration or cleanup needed!
	// The dynamic reconfiguration system will:
	// 1. Auto-detect current cluster state (or use defaults if fresh cluster)
	// 2. Reconfigure per-test based on test labels
	// 3. Next test run will detect state and reconfigure as needed - no cleanup required!
	if err := suite.BeforeSuiteFunc(); err != nil {
		log.Printf("Setup failed: %v", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}
