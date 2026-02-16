package kubeprep

import (
	"context"
)

// TestingT is the interface that wraps the methods we need from *testing.T.
// This allows kubeprep functions to work with testing.T and use Gomega matchers.
type TestingT interface {
	Context() context.Context
	Helper()
	Log(args ...any)
	Logf(format string, args ...any)
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
	FailNow()
}
