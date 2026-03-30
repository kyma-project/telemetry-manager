package selfmonitor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	selfmonports "github.com/kyma-project/telemetry-manager/internal/selfmonitor/ports"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

// logSelfMonitorMetrics queries the self-monitor Prometheus instant query API for all
// metrics relevant to the given component and logs their current values for diagnostics.
// It never fails the test.
func logSelfMonitorMetrics(t *testing.T, ctx context.Context, component string) {
	t.Helper()

	t.Logf("--- self-monitor metrics [%s] ---", time.Now().Format(time.TimeOnly))

	for _, query := range metricsForComponent(component) {
		value, err := queryPrometheus(ctx, query)
		if err != nil {
			t.Logf("selfmon metric query failed [%s]: %v", query, err)
			continue
		}

		t.Logf("selfmon metric [%s]: %s", query, value)
	}
}

// alertConditionDescription returns a human-readable description of the metric condition
// that must be satisfied for the given condition reason to be set.
// component is needed because Fluent Bit uses different metric names than OTel for the same reason.
func alertConditionDescription(reason, component string) string {
	isFluentBitComponent := component == suite.LabelFluentBit

	switch reason {
	case "AgentAllTelemetryDataDropped":
		if isFluentBitComponent {
			return "need: rate(fluentbit_output_dropped_records) > 0 AND rate(fluentbit_output_proc_bytes) == 0"
		}

		return "need: rate(send_failed or enqueue_failed) > 0 AND rate(sent) == 0"
	case "GatewayAllTelemetryDataDropped":
		return "need: rate(send_failed or enqueue_failed) > 0 AND rate(sent) == 0"
	case "AgentSomeTelemetryDataDropped", "AgentTelemetryDataDropped":
		if isFluentBitComponent {
			return "need: rate(fluentbit_output_dropped_records) > 0 AND rate(fluentbit_output_proc_bytes) > 0"
		}

		return "need: rate(send_failed or enqueue_failed) > 0 AND rate(sent) > 0"
	case "GatewaySomeTelemetryDataDropped", "GatewayTelemetryDataDropped":
		return "need: rate(send_failed or enqueue_failed) > 0 AND rate(sent) > 0"
	case "AgentBufferFillingUp":
		return "need: max(fluentbit_input_storage_chunks_down) > 300"
	case "AgentNoLogsDelivered":
		return "need: rate(fluentbit_input_bytes) > 0 AND rate(fluentbit_output_proc_bytes) == 0"
	case "GatewayThrottling":
		return "need: rate(receiver_refused) > 0"
	default:
		return ""
	}
}

// metricsForComponent returns the PromQL instant queries relevant to the alert
// conditions for the given component. Queries use rate(...[5m]) to match the
// actual alert rule expressions, so the logged values directly explain whether
// the alert condition is met.
func metricsForComponent(component string) []string {
	switch component {
	case suite.LabelLogAgent:
		svc := `service="telemetry-log-agent-metrics"`

		return []string{
			`sum by (pipeline_name) (rate(otelcol_exporter_send_failed_log_records_total{` + svc + `}[5m]))`,
			`sum by (pipeline_name) (rate(otelcol_exporter_enqueue_failed_log_records_total{` + svc + `}[5m]))`,
			`sum by (pipeline_name) (rate(otelcol_exporter_sent_log_records_total{` + svc + `}[5m]))`,
		}
	case suite.LabelLogGateway:
		svc := `service="telemetry-log-gateway-metrics"`

		return []string{
			`sum by (pipeline_name) (rate(otelcol_exporter_send_failed_log_records_total{` + svc + `}[5m]))`,
			`sum by (pipeline_name) (rate(otelcol_exporter_enqueue_failed_log_records_total{` + svc + `}[5m]))`,
			`sum by (pipeline_name) (rate(otelcol_exporter_sent_log_records_total{` + svc + `}[5m]))`,
		}
	case suite.LabelFluentBit:
		svc := `service="telemetry-fluent-bit-metrics"`

		return []string{
			`sum by (pipeline_name) (rate(fluentbit_output_dropped_records_total{` + svc + `}[5m]))`,
			`sum by (pipeline_name) (rate(fluentbit_output_proc_bytes_total{` + svc + `}[5m]))`,
			`sum by (pipeline_name) (rate(fluentbit_input_bytes_total{` + svc + `}[5m]))`,
			`max by (pipeline_name) (fluentbit_input_storage_chunks_down{` + svc + `})`,
		}
	case suite.LabelMetricGateway:
		svc := `service="telemetry-metric-gateway-metrics"`

		return []string{
			`sum by (pipeline_name) (rate(otelcol_exporter_send_failed_metric_points_total{` + svc + `}[5m]))`,
			`sum by (pipeline_name) (rate(otelcol_exporter_enqueue_failed_metric_points_total{` + svc + `}[5m]))`,
			`sum by (pipeline_name) (rate(otelcol_exporter_sent_metric_points_total{` + svc + `}[5m]))`,
		}
	case suite.LabelMetricAgent:
		svc := `service="telemetry-metric-agent-metrics"`

		return []string{
			`sum by (pipeline_name) (rate(otelcol_exporter_send_failed_metric_points_total{` + svc + `}[5m]))`,
			`sum by (pipeline_name) (rate(otelcol_exporter_enqueue_failed_metric_points_total{` + svc + `}[5m]))`,
			`sum by (pipeline_name) (rate(otelcol_exporter_sent_metric_points_total{` + svc + `}[5m]))`,
		}
	case suite.LabelTraces:
		svc := `service="telemetry-trace-gateway-metrics"`

		return []string{
			`sum by (pipeline_name) (rate(otelcol_exporter_send_failed_spans_total{` + svc + `}[5m]))`,
			`sum by (pipeline_name) (rate(otelcol_exporter_enqueue_failed_spans_total{` + svc + `}[5m]))`,
			`sum by (pipeline_name) (rate(otelcol_exporter_sent_spans_total{` + svc + `}[5m]))`,
		}
	default:
		return nil
	}
}

// logSelfMonitorTargets queries the self-monitor Prometheus targets API and logs
// all discovered scrape targets (active and dropped) with their health status.
// It never fails the test.
func logSelfMonitorTargets(t *testing.T, ctx context.Context) {
	t.Helper()

	active, dropped, err := queryPrometheusTargets(ctx)
	if err != nil {
		t.Logf("selfmon targets query failed: %v", err)
		return
	}

	t.Logf("--- self-monitor targets (active: %d, dropped: %d) ---", len(active), len(dropped))

	for _, target := range active {
		t.Logf("  [%s] %s (labels: %v)", target.health, target.scrapeURL, target.labels)
	}

	for _, target := range dropped {
		t.Logf("  [dropped] %s (labels: %v)", target.scrapeURL, target.labels)
	}
}

// assertSelfMonitorHasActiveTargets waits until Prometheus has discovered at least one active
// scrape target. This guards against flakiness caused by the Prometheus kubernetes SD
// watch-list failing during API server startup — if SD never connects, no targets are
// discovered and no metrics are ever scraped.
// If no targets are found within 1 minute, the selfmonitor pod is restarted once: a fresh
// network namespace picks up all iptables/CNI rules that were not yet ready at initial pod start.
func assertSelfMonitorHasActiveTargets(t *testing.T) {
	t.Helper()

	hasActiveTargets := func() bool {
		active, _, err := queryPrometheusTargets(t.Context())
		if err != nil {
			t.Logf("targets query failed (SD not ready yet): %v", err)
			return false
		}

		if len(active) == 0 {
			t.Logf("no active targets yet (kubernetes SD may still be connecting to API server)")
			return false
		}

		t.Logf("prometheus SD ready: %d active targets discovered", len(active))

		return true
	}

	// First, poll for up to 1 minute. If no targets appear, restart the pod to recover
	// from a potential CNI/iptables race where the pod's network namespace was set up
	// before k3s had fully programmed the kubernetes ClusterIP DNAT rules.
	deadline := time.Now().Add(time.Minute)
	for time.Now().Before(deadline) {
		if hasActiveTargets() {
			return
		}
		time.Sleep(periodic.SelfmonitorQueryInterval)
	}

	t.Log("No active targets after 1 minute, restarting selfmonitor pod to recover from potential CNI/iptables race")
	restartSelfMonitorPod(t)

	// Now wait the full timeout for targets to appear after restart.
	Eventually(hasActiveTargets, periodic.SelfmonitorRateBaselineTimeout, periodic.SelfmonitorQueryInterval).Should(
		BeTrue(),
		"prometheus service discovery never discovered any targets",
	)
}

// logScrapeEndpoints lists all Endpoints in the scrape namespace that carry the
// self-monitor label, so we can see whether targets are missing because k8s
// Endpoints don't exist yet or because Prometheus SD isn't discovering them.
// It never fails the test.
func logScrapeEndpoints(t *testing.T, ctx context.Context) {
	t.Helper()

	var epList corev1.EndpointsList
	if err := suite.K8sClient.List(ctx, &epList,
		client.InNamespace(kitkyma.SystemNamespaceName),
		client.MatchingLabels{
			commonresources.LabelKeyTelemetrySelfMonitor: commonresources.LabelValueTelemetrySelfMonitor,
		},
	); err != nil {
		t.Logf("  endpoints list error: %v", err)
		return
	}

	t.Logf("--- scrape endpoints in %s (self-monitor label, count: %d) ---", kitkyma.SystemNamespaceName, len(epList.Items))

	for i := range epList.Items {
		ep := &epList.Items[i]
		for _, sub := range ep.Subsets {
			for _, addr := range sub.Addresses {
				for _, port := range sub.Ports {
					t.Logf("  %s: %s:%d (ready)", ep.Name, addr.IP, port.Port)
				}
			}

			for _, addr := range sub.NotReadyAddresses {
				for _, port := range sub.Ports {
					t.Logf("  %s: %s:%d (not-ready)", ep.Name, addr.IP, port.Port)
				}
			}
		}

		if len(ep.Subsets) == 0 {
			t.Logf("  %s: no subsets", ep.Name)
		}
	}
}

type promTarget struct {
	scrapeURL string
	health    string
	labels    map[string]string
}

type promTargetsResponse struct {
	Status string `json:"status"`
	Data   struct {
		ActiveTargets []struct {
			ScrapeURL string            `json:"scrapeUrl"`
			Health    string            `json:"health"`
			Labels    map[string]string `json:"labels"`
			LastError string            `json:"lastError"`
		} `json:"activeTargets"`
		DroppedTargets []struct {
			DiscoveredLabels map[string]string `json:"discoveredLabels"`
		} `json:"droppedTargets"`
	} `json:"data"`
}

func queryPrometheusTargets(ctx context.Context) (active, dropped []promTarget, err error) {
	baseURL := suite.ProxyClient.ProxyURLForService(
		kitkyma.SelfMonitorService.Namespace,
		kitkyma.SelfMonitorService.Name,
		"api/v1/targets",
		selfmonports.PrometheusPort,
	)

	resp, err := suite.ProxyClient.GetWithContext(ctx, baseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("GET failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read body failed: %w", err)
	}

	var result promTargetsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, nil, fmt.Errorf("parse failed: %w", err)
	}

	if result.Status != "success" {
		return nil, nil, fmt.Errorf("prometheus status: %s", result.Status)
	}

	active = make([]promTarget, len(result.Data.ActiveTargets))
	for i, at := range result.Data.ActiveTargets {
		health := at.Health
		if at.LastError != "" {
			health += " (" + at.LastError + ")"
		}

		active[i] = promTarget{
			scrapeURL: at.ScrapeURL,
			health:    health,
			labels:    at.Labels,
		}
	}

	dropped = make([]promTarget, len(result.Data.DroppedTargets))
	for i, dt := range result.Data.DroppedTargets {
		dropped[i] = promTarget{
			scrapeURL: dt.DiscoveredLabels["__address__"],
			labels:    dt.DiscoveredLabels,
		}
	}

	return active, dropped, nil
}

type promQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]any            `json:"value"` // [timestamp, "value"]
		} `json:"result"`
	} `json:"data"`
}

func queryPrometheus(ctx context.Context, query string) (string, error) {
	baseURL := suite.ProxyClient.ProxyURLForService(
		kitkyma.SelfMonitorService.Namespace,
		kitkyma.SelfMonitorService.Name,
		"api/v1/query",
		selfmonports.PrometheusPort,
	)

	queryURL := baseURL + "?query=" + url.QueryEscape(query)

	resp, err := suite.ProxyClient.GetWithContext(ctx, queryURL)
	if err != nil {
		return "", fmt.Errorf("GET failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body failed: %w", err)
	}

	var result promQueryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse failed: %w", err)
	}

	if result.Status != "success" {
		return "", fmt.Errorf("prometheus status: %s", result.Status)
	}

	if len(result.Data.Result) == 0 {
		return "(no data)", nil
	}

	var sb strings.Builder

	for _, r := range result.Data.Result {
		fmt.Fprintf(&sb, "%v=%v ", r.Metric, r.Value[1])
	}

	return sb.String(), nil
}

// logSelfMonitorPodLogs fetches and logs the last lines of the selfmonitor Prometheus container logs.
// It never fails the test.
func logSelfMonitorPodLogs(t *testing.T, ctx context.Context) {
	t.Helper()

	var podList corev1.PodList
	if err := suite.K8sClient.List(ctx, &podList,
		client.InNamespace(kitkyma.SystemNamespaceName),
		client.MatchingLabels{commonresources.LabelKeyK8sName: names.SelfMonitor},
	); err != nil {
		t.Logf("selfmon pod list error: %v", err)
		return
	}

	t.Logf("--- selfmonitor pod logs (count: %d) ---", len(podList.Items))

	for i := range podList.Items {
		pod := &podList.Items[i]

		logURL := fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s/log?container=%s&tailLines=100",
			suite.ProxyClient.APIServerURL(),
			pod.Namespace,
			pod.Name,
			names.SelfMonitorContainerName,
		)

		resp, err := suite.ProxyClient.GetWithContext(ctx, logURL)
		if err != nil {
			t.Logf("  pod %s: log fetch error: %v", pod.Name, err)
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Logf("  pod %s: read error: %v", pod.Name, err)
			continue
		}

		t.Logf("  pod %s logs:\n%s", pod.Name, string(body))
	}
}

// restartSelfMonitorPod deletes all selfmonitor pods, causing the Deployment to recreate them
// with a fresh network namespace. Used as a mitigation when Prometheus kubernetes SD fails to
// connect to the API server due to a CNI/iptables race at pod startup time.
func restartSelfMonitorPod(t *testing.T) {
	t.Helper()

	var podList corev1.PodList
	if err := suite.K8sClient.List(t.Context(), &podList,
		client.InNamespace(kitkyma.SystemNamespaceName),
		client.MatchingLabels{commonresources.LabelKeyK8sName: names.SelfMonitor},
	); err != nil {
		t.Logf("selfmon pod list error (skipping restart): %v", err)
		return
	}

	for i := range podList.Items {
		pod := &podList.Items[i]
		if err := suite.K8sClient.Delete(t.Context(), pod); client.IgnoreNotFound(err) != nil {
			t.Logf("failed to delete selfmon pod %s (skipping): %v", pod.Name, err)
		} else {
			t.Logf("deleted selfmon pod %s for restart", pod.Name)
		}
	}

	// Give the new pod time to start before the caller retries target discovery.
	time.Sleep(10 * time.Second)
}
