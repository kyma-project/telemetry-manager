package assert

import "context"

// TestingT is a temporary interface that abstracts over different test frameworks.
// It allows the use of both testing.T (for migrated tests) and GinkgoT (for tests not yet migrated).
type TestingT interface {
	Context() context.Context
	Helper()
}
