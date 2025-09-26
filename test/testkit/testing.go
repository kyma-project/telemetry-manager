package testkit

import "context"

// T is a temporary interface that abstracts over different test frameworks.
// It allows the use of both testing.T (for migrated tests) and GinkgoT (for tests not yet migrated).
// TODO(TeodorSAP): To be replaced with testing.T after migration is done
type T interface {
	Context() context.Context
	Helper()
	Logf(format string, args ...any)
}
