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
	const errorCode = 1

	if err := suite.BeforeSuiteFunc(); err != nil {
		log.Printf("Setup failed: %v", err)
		os.Exit(errorCode)
	}

	m.Run()
}
