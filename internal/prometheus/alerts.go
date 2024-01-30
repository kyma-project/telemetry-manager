package prometheus

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

const prometheusAPIURL = "http://prometheus-server.default:80"

//var criticalAlerts = []string{"ExporterDroppedMetrics", "ReceiverDroppedMetrics", "ExporterDroppedSpans", "ReceiverDroppedSpans", "ReceiverDroppedLogs"}

type Alerts struct {
	Name         string
	Severity     string
	PipelineInfo string
}

func NewAlerts() Alerts {
	return Alerts{
		Name:         "",
		Severity:     "",
		PipelineInfo: "",
	}
}

func SetUnknownAlert() Alerts {
	return Alerts{
		Name:         "Unknown",
		Severity:     "Unknown",
		PipelineInfo: "Unknown",
	}
}

func QueryAlerts(ctx context.Context, currentAlert Alerts) (error, Alerts) {
	client, err := api.NewClient(api.Config{
		Address: prometheusAPIURL,
	})
	if err != nil {
		return fmt.Errorf("failed to create Prometheus client: %w", err), Alerts{}
	}

	v1api := promv1.NewAPI(client)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	start := time.Now()
	alerts, err := v1api.Alerts(ctx)

	if err != nil {
		return fmt.Errorf("failed to query Prometheus alerts: %w", err), Alerts{}
	}

	logf.FromContext(ctx).Info("Prometheus alert query succeeded!",
		"elapsed_ms", time.Since(start).Milliseconds(),
		"alerts", alerts)
	if len(alerts.Alerts) == 0 {
		return nil, Alerts{}
	}

	alert := fetchAlert(alerts, currentAlert)
	return nil, alert
}

func fetchAlert(alerts promv1.AlertsResult, currentAlert Alerts) Alerts {
	if len(alerts.Alerts) == 0 {
		return Alerts{}
	}
	firingAlerts := fetchFiringAlerts(alerts.Alerts)
	// Verify if current Alert is still firing and if critical then dont change the state
	if currentCriticalAlertIsStillFiring(currentAlert, firingAlerts) {
		return currentAlert
	}
	//if currentAlert.Name != "" && firingAlertsContainsAlert(currentAlert.Name, firingAlerts) {
	//	if slices.Contains(criticalAlerts, currentAlert.Name) {
	//		return currentAlert
	//	}
	//}
	alert := fetchCriticalAlerts(firingAlerts)
	if alert.Name != "" {
		return alert
	}
	return fetchNonCriticalAlerts(firingAlerts)
}

func currentCriticalAlertIsStillFiring(currentAlert Alerts, firingAlerts []promv1.Alert) bool {
	if currentAlert.Name == "" {
		return false
	}
	if currentAlert.Severity == "critical" && firingAlertsContainsAlert(currentAlert.Name, firingAlerts) {
		return true
	}
	return false
}

func firingAlertsContainsAlert(alertName string, alerts []promv1.Alert) bool {
	for _, alert := range alerts {
		if string(alert.Labels["alertname"]) == alertName {
			return true
		}
	}
	return false
}
func fetchFiringAlerts(alerts []promv1.Alert) []promv1.Alert {
	var firingAlerts []promv1.Alert
	for _, alert := range alerts {
		if alert.State == promv1.AlertStateFiring {
			firingAlerts = append(firingAlerts, alert)
		}
	}
	return firingAlerts
}
func fetchCriticalAlerts(alerts []promv1.Alert) Alerts {
	for _, alert := range alerts {
		if string(alert.Labels["severity"]) == "critical" {
			return Alerts{
				Name:     string(alert.Labels["alertname"]),
				Severity: string(alert.Labels["severity"]),
			}
		}
	}
	return Alerts{}
}

func fetchNonCriticalAlerts(alerts []promv1.Alert) Alerts {
	for _, alert := range alerts {
		return Alerts{
			Name:         string(alert.Labels["alertname"]),
			Severity:     string(alert.Labels["severity"]),
			PipelineInfo: FetchPipelineInfo(alert),
		}
	}
	return Alerts{}
}

func FetchPipelineInfo(alert promv1.Alert) string {
	if string(alert.Labels["alertname"]) == "ExporterDropsMetric" || string(alert.Labels["alertname"]) == "ExporterDropsSpans" || string(alert.Labels["alertname"]) == "ExporterDropsLogs" {
		return string(alert.Labels["exporter"])
	}
	if string(alert.Labels["alertname"]) == "ReceiverDropsMetric" || string(alert.Labels["alertname"]) == "ReceiverDropsSpans" || string(alert.Labels["alertname"]) == "ReceiverDropsLogs" {
		return string(alert.Labels["receiver"])
	}
	return ""
}
