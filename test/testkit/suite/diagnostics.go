package suite

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
)

const systemNamespace = "kyma-system"

// artifactsBaseDir returns the root directory for test artifacts.
// It uses the TEST_ARTIFACTS_DIR env var if set, otherwise falls back to
// "test-artifacts" relative to the working directory.
// When running via `make e2e-test` the Makefile sets TEST_ARTIFACTS_DIR to
// $(CURDIR)/test-artifacts so the path is always under the repo root.
func artifactsBaseDir() string {
	if dir := os.Getenv("TEST_ARTIFACTS_DIR"); dir != "" {
		return dir
	}

	return "test-artifacts"
}

// TestArtifactsDir returns the directory where test artifacts (logs, YAMLs) for the
// given test should be written. The path is <artifactsBaseDir>/<sanitized-test-name>.
// The directory is NOT created by this function — callers must create it as needed.
func TestArtifactsDir(t *testing.T) string {
	t.Helper()

	name := strings.NewReplacer("/", "_", " ", "_", ":", "_").Replace(t.Name())

	return filepath.Join(artifactsBaseDir(), name)
}

// writeToFile writes content to a file at dir/filename.
// If the file cannot be created the error is logged but the test is not failed.
func writeToFile(t *testing.T, dir, filename, content string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Logf("artifacts dir create error (%s): %v", dir, err)
		return
	}

	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Logf("artifact write error (%s): %v", path, err)
	}
}

// WriteArtifact writes content to dir/filename. The directory is created if it does not exist.
// Errors are logged via t.Logf but never fail the test. Use this from packages outside suite
// to avoid duplicating the permission constants (which would trigger the mnd linter).
func WriteArtifact(t *testing.T, dir, filename, content string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Logf("artifacts dir create error (%s): %v", dir, err)
		return
	}

	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Logf("artifact write error (%s): %v", path, err)
	}
}

// registerDiagnosticsOnFailure registers a t.Cleanup that dumps pod overview,
// telemetry-manager logs, and resource YAMLs when the test fails. It is called
// from SetupTestWithOptions so every E2E test gets this for free.
//
// Because t.Cleanup runs in LIFO order, registering here (early in SetupTestWithOptions)
// means this cleanup runs *last* — after all per-test resource deletions. That is acceptable:
// the manager pod persists beyond pipeline CRD cleanup, so its logs are still available.
func registerDiagnosticsOnFailure(t *testing.T) {
	t.Helper()

	t.Cleanup(func() {
		if !t.Failed() {
			return
		}

		dir := TestArtifactsDir(t)
		ctx := context.Background()
		collectPodsOverview(t, ctx, dir)
		collectTelemetryManagerPodLogs(t, ctx, dir)
	})
}

// collectPodsOverview writes a one-line summary for every pod across all namespaces
// to both t.Logf and a file. Never fails the test.
func collectPodsOverview(t *testing.T, ctx context.Context, dir string) {
	t.Helper()

	var podList corev1.PodList
	if err := K8sClient.List(ctx, &podList); err != nil {
		t.Logf("pod overview list error: %v", err)
		return
	}

	var sb strings.Builder

	fmt.Fprintf(&sb, "pod count: %d\n", len(podList.Items))

	for i := range podList.Items {
		pod := &podList.Items[i]
		readyCount := 0

		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Ready {
				readyCount++
			}
		}

		fmt.Fprintf(&sb, "  %s/%s  phase=%s  ready=%d/%d\n",
			pod.Namespace, pod.Name,
			pod.Status.Phase,
			readyCount, len(pod.Spec.Containers),
		)
	}

	writeToFile(t, dir, "pod-overview.txt", sb.String())
}

// collectTelemetryManagerPodLogs fetches all available logs from each container in the
// telemetry-manager pod and writes them to both t.Logf and a file. Never fails the test.
func collectTelemetryManagerPodLogs(t *testing.T, ctx context.Context, dir string) {
	t.Helper()

	var podList corev1.PodList
	if err := K8sClient.List(ctx, &podList,
		client.InNamespace(systemNamespace),
		client.MatchingLabels{
			commonresources.LabelKeyK8sName:    "manager",
			commonresources.LabelKeyKymaModule: commonresources.LabelValueKymaModule,
		},
	); err != nil {
		t.Logf("telemetry-manager pod list error: %v", err)
		return
	}

	var sb strings.Builder

	for i := range podList.Items {
		pod := &podList.Items[i]

		for _, container := range pod.Spec.Containers {
			logURL := fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s/log?container=%s",
				ProxyClient.APIServerURL(),
				pod.Namespace,
				pod.Name,
				container.Name,
			)

			resp, err := ProxyClient.GetWithContext(ctx, logURL)
			if err != nil {
				fmt.Fprintf(&sb, "pod %s container %s: log fetch error: %v\n", pod.Name, container.Name, err)
				continue
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Fprintf(&sb, "pod %s container %s: read error: %v\n", pod.Name, container.Name, err)
				continue
			}

			fmt.Fprintf(&sb, "=== pod %s container %s ===\n%s\n", pod.Name, container.Name, string(body))
		}
	}

	writeToFile(t, dir, "manager-logs.txt", sb.String())
}
