package kubeprep

import (
	"context"
)

// TestingT is the interface that wraps the methods we need from *testing.T.
// This allows kubeprep functions to use t.Context() and t.Log() directly.
type TestingT interface {
	Context() context.Context
	Helper()
	Log(args ...any)
	Logf(format string, args ...any)
}
