package shared

import (
	"fmt"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	. "github.com/onsi/gomega"
	
	"testing"
)

func TestMain(m *testing.M) {
	RegisterFailHandler(func(message string, callerSkip ...int) {
		// This will make Gomega assertions fail the test
		// Here we just panic, but you could also hook into testing.T explicitly if needed
		panic(fmt.Sprintf("Gomega assertion failed: %s", message))
	})
	suite.BeforeSuiteFunc()
	m.Run()
	suite.AfterSuiteFunc()
}
