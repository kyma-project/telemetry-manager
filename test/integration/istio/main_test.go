package istio

import (
	"log"
	"os"
	"testing"

	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

// creating mocks in a specially prepared namespace that allows calling workloads in the mesh via API server proxy
const permissiveNs = "istio-permissive-mtls"

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

	// Run tests - cluster state changes during test execution
	exitCode := m.Run()

	// No cleanup needed! Next test run will detect and reconfigure automatically.
	// This makes test runs idempotent and eliminates cleanup failures.

	os.Exit(exitCode)
}
