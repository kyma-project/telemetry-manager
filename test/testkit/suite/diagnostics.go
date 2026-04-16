package suite

import (
	"context"
	"fmt"
	"io"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
)

const systemNamespace = "kyma-system"

// registerDiagnosticsOnFailure registers a t.Cleanup that dumps pod overview and
// telemetry-manager logs when the test fails. It is called from SetupTestWithOptions
// so every E2E test gets this for free.
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

		ctx := context.Background()
		logPodsOverview(t, ctx)
		logTelemetryManagerPodLogs(t, ctx)
	})
}

// logPodsOverview logs a one-line summary for every pod across all namespaces.
// Never fails the test.
func logPodsOverview(t *testing.T, ctx context.Context) {
	t.Helper()

	var podList corev1.PodList
	if err := K8sClient.List(ctx, &podList); err != nil {
		t.Logf("pod overview list error: %v", err)
		return
	}

	t.Logf("--- pod overview (all namespaces, count: %d) ---", len(podList.Items))

	for i := range podList.Items {
		pod := &podList.Items[i]
		readyCount := 0

		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Ready {
				readyCount++
			}
		}

		t.Logf("  %s/%s  phase=%s  ready=%d/%d",
			pod.Namespace, pod.Name,
			pod.Status.Phase,
			readyCount, len(pod.Spec.Containers),
		)
	}
}

// logTelemetryManagerPodLogs fetches the last 100 lines from each container in the
// telemetry-manager pod. Never fails the test.
func logTelemetryManagerPodLogs(t *testing.T, ctx context.Context) {
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

	t.Logf("--- telemetry-manager pod logs (count: %d) ---", len(podList.Items))

	for i := range podList.Items {
		pod := &podList.Items[i]

		for _, container := range pod.Spec.Containers {
			logURL := fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s/log?container=%s&tailLines=100",
				ProxyClient.APIServerURL(),
				pod.Namespace,
				pod.Name,
				container.Name,
			)

			resp, err := ProxyClient.GetWithContext(ctx, logURL)
			if err != nil {
				t.Logf("  pod %s container %s: log fetch error: %v", pod.Name, container.Name, err)
				continue
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Logf("  pod %s container %s: read error: %v", pod.Name, container.Name, err)
				continue
			}

			t.Logf("  pod %s container %s logs:\n%s", pod.Name, container.Name, string(body))
		}
	}
}
