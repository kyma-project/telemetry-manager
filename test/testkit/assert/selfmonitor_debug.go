package assert

import (
	"fmt"
	"io"
	"net/http"
	"testing"

	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

// SelfMonitorDebugOnFailure registers a t.Cleanup function that dumps self-monitor
// Prometheus diagnostic information (targets, alerts, rules) when the test fails.
// This helps diagnose why self-monitor alerts are not firing in CI environments.
func SelfMonitorDebugOnFailure(t *testing.T) {
	t.Helper()

	t.Cleanup(func() {
		if !t.Failed() {
			return
		}

		t.Log("=== Self-Monitor Debug Dump (test failed) ===")

		endpoints := []struct {
			name string
			path string
		}{
			{"Targets", "api/v1/targets"},
			{"Alerts", "api/v1/alerts"},
			{"Rules", "api/v1/rules"},
		}

		for _, ep := range endpoints {
			body, err := querySelfMonitor(t, ep.path)
			if err != nil {
				t.Logf("Self-Monitor %s: error querying: %v", ep.name, err)
				continue
			}

			t.Logf("Self-Monitor %s:\n%s", ep.name, body)
		}
	})
}

func querySelfMonitor(t *testing.T, path string) (string, error) {
	t.Helper()

	url := suite.ProxyClient.ProxyURLForService(
		kitkyma.SelfMonitorName.Namespace,
		kitkyma.SelfMonitorName.Name,
		path,
		9090,
	)

	resp, err := suite.ProxyClient.GetWithContext(t.Context(), url)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}

	return string(body), nil
}
