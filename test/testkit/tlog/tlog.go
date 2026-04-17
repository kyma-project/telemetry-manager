// Package tlog provides a test logger that writes timestamped messages directly
// to a file under the test artifacts directory instead of using t.Logf (which
// always prints to stdout). The log file is created lazily on first write and
// flushed after every call. A t.Cleanup is registered automatically to close it.
package tlog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Logf writes a timestamped, formatted message to <artifactsDir>/<testName>/test.log.
// It never calls t.Logf, so nothing is printed to stdout.
func Logf(t *testing.T, format string, args ...any) {
	t.Helper()
	w := writerFor(t)
	msg := fmt.Sprintf("[%s] %s\n", time.Now().UTC().Format(time.RFC3339), fmt.Sprintf(format, args...))
	w.write(msg)
}

// --- internals ---

var (
	mu      sync.Mutex
	writers = map[string]*fileWriter{}
)

type fileWriter struct {
	mu   sync.Mutex
	file *os.File
}

func (w *fileWriter) write(msg string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return
	}

	//nolint:errcheck // best-effort log write
	_, _ = w.file.WriteString(msg)
}

func (w *fileWriter) close() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
}

func writerFor(t *testing.T) *fileWriter {
	t.Helper()

	key := t.Name()

	mu.Lock()
	w, ok := writers[key]
	if !ok {
		w = &fileWriter{}
		writers[key] = w

		dir := artifactDir(t)
		if err := os.MkdirAll(dir, 0o700); err == nil {
			path := filepath.Join(dir, "test.log")
			if f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
				w.file = f
			}
		}

		t.Cleanup(func() {
			mu.Lock()
			delete(writers, key)
			mu.Unlock()
			w.close()
		})
	}
	mu.Unlock()

	return w
}

// artifactDir mirrors the logic in suite.TestArtifactsDir to avoid an import cycle.
func artifactDir(t *testing.T) string {
	base := os.Getenv("TEST_ARTIFACTS_DIR")
	if base == "" {
		base = "test-artifacts"
	}

	name := strings.NewReplacer("/", "_", " ", "_", ":", "_").Replace(t.Name())

	return filepath.Join(base, name)
}
