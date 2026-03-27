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

	selfmonports "github.com/kyma-project/telemetry-manager/internal/selfmonitor/ports"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

// logSelfMonitorMetrics queries the self-monitor Prometheus instant query API for all
// metrics relevant to the given component and logs their current values for diagnostics.
// It never fails the test.
func logSelfMonitorMetrics(t *testing.T, component string) {
	t.Helper()

	t.Logf("--- self-monitor metrics [%s] ---", time.Now().Format(time.TimeOnly))

	for _, query := range metricsForComponent(component) {
		value, err := queryPrometheus(t.Context(), query)
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
// the discovered scrape targets with their health status. It never fails the test.
func logSelfMonitorTargets(t *testing.T) {
	t.Helper()

	targets, err := queryPrometheusTargets(t.Context())
	if err != nil {
		t.Logf("selfmon targets query failed: %v", err)
		return
	}

	t.Logf("--- self-monitor targets (%d active) ---", len(targets))

	for _, target := range targets {
		t.Logf("  [%s] %s (labels: %v)", target.health, target.scrapeURL, target.labels)
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
	} `json:"data"`
}

func queryPrometheusTargets(ctx context.Context) ([]promTarget, error) {
	baseURL := suite.ProxyClient.ProxyURLForService(
		kitkyma.SelfMonitorService.Namespace,
		kitkyma.SelfMonitorService.Name,
		"api/v1/targets",
		selfmonports.PrometheusPort,
	)

	resp, err := suite.ProxyClient.GetWithContext(ctx, baseURL)
	if err != nil {
		return nil, fmt.Errorf("GET failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body failed: %w", err)
	}

	var result promTargetsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse failed: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("prometheus status: %s", result.Status)
	}

	targets := make([]promTarget, len(result.Data.ActiveTargets))
	for i, at := range result.Data.ActiveTargets {
		health := at.Health
		if at.LastError != "" {
			health += " (" + at.LastError + ")"
		}

		targets[i] = promTarget{
			scrapeURL: at.ScrapeURL,
			health:    health,
			labels:    at.Labels,
		}
	}

	return targets, nil
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
