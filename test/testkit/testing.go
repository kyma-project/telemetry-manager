package testkit

import "context"

// TODO(TeodorSAP): To be replaced with testing.T after migration is done
// T is a temporary interface that abstracts over different test frameworks.
// It allows the use of both testing.T (for migrated tests) and GinkgoT (for tests not yet migrated).
type T interface {
	Context() context.Context
	Helper()
}
