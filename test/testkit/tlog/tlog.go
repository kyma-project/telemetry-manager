// Package tlog provides a timestamped wrapper around t.Logf.
package tlog

import (
	"fmt"
	"testing"
	"time"
)

// Logf logs a formatted message with an RFC3339 timestamp prefix via t.Logf.
func Logf(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Logf("[%s] %s", time.Now().UTC().Format(time.RFC3339), fmt.Sprintf(format, args...))
}
