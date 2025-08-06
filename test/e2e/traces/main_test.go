package traces

import (
	"log"
	"os"
	"testing"

	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestMain(m *testing.M) {
	const errorCode = 1

	if err := suite.BeforeSuiteFuncErr(); err != nil {
		log.Printf("Setup failed: %v", err)
		os.Exit(errorCode)
	}

	m.Run()
}
