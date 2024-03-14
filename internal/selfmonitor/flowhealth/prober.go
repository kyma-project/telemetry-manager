package flowhealth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

const (
	selfMonitorAPIURL = "http://telemetry-self-monitor.kyma-system:9090"
	clientTimeout     = 10 * time.Second
)

//go:generate mockery --name alertGetter --filename=alert_getter.go --exported
type alertGetter interface {
	Alerts(ctx context.Context) (promv1.AlertsResult, error)
}

type Prober struct {
	getter        alertGetter
	clientTimeout time.Duration
}

func NewProber() (*Prober, error) {
	client, err := api.NewClient(api.Config{
		Address: selfMonitorAPIURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus client: %w", err)
	}

	return &Prober{
		getter:        promv1.NewAPI(client),
		clientTimeout: clientTimeout,
	}, nil
}

type ProbeResult struct {
	AllDataDropped  bool
	SomeDataDropped bool
	QueueAlmostFull bool
	Throttling      bool
	Healthy         bool
}

func (p *Prober) Probe(ctx context.Context, pipelineName string) (ProbeResult, error) {
	alerts, err := p.retrieveAlerts(ctx)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("failed to retrieve alerts: %w", err)
	}

	return ProbeResult{
		AllDataDropped:  allDataDropped(alerts, pipelineName),
		SomeDataDropped: someDataDropped(alerts, pipelineName),
		QueueAlmostFull: queueAlmostFull(alerts, pipelineName),
		Throttling:      throttling(alerts),
		Healthy:         healthy(alerts, pipelineName),
	}, nil
}

func allDataDropped(alerts []promv1.Alert, pipelineName string) bool {
	exporterSentFiring := hasFiringExporterAlert(alerts, alertNameExporterSentData, pipelineName)
	exporterDroppedFiring := hasFiringExporterAlert(alerts, alertNameExporterDroppedData, pipelineName)
	exporterEnqueueFailedFiring := hasFiringExporterAlert(alerts, alertNameExporterEnqueueFailed, pipelineName)

	return !exporterSentFiring && (exporterDroppedFiring || exporterEnqueueFailedFiring)
}

func someDataDropped(alerts []promv1.Alert, pipelineName string) bool {
	exporterSentFiring := hasFiringExporterAlert(alerts, alertNameExporterSentData, pipelineName)
	exporterDroppedFiring := hasFiringExporterAlert(alerts, alertNameExporterDroppedData, pipelineName)
	exporterEnqueueFailedFiring := hasFiringExporterAlert(alerts, alertNameExporterEnqueueFailed, pipelineName)

	return exporterSentFiring && (exporterDroppedFiring || exporterEnqueueFailedFiring)
}

func queueAlmostFull(alerts []promv1.Alert, pipelineName string) bool {
	return hasFiringExporterAlert(alerts, alertNameExporterQueueAlmostFull, pipelineName)
}

func throttling(alerts []promv1.Alert) bool {
	return hasFiringAlert(alerts, alertNameReceiverRefusedData)
}

func healthy(alerts []promv1.Alert, pipelineName string) bool {
	return !(hasFiringExporterAlert(alerts, alertNameExporterDroppedData, pipelineName) ||
		hasFiringExporterAlert(alerts, alertNameExporterQueueAlmostFull, pipelineName) ||
		hasFiringExporterAlert(alerts, alertNameExporterEnqueueFailed, pipelineName) ||
		hasFiringAlert(alerts, alertNameReceiverRefusedData))
}

func (p *Prober) retrieveAlerts(ctx context.Context) ([]promv1.Alert, error) {
	childCtx, cancel := context.WithTimeout(ctx, p.clientTimeout)
	defer cancel()

	result, err := p.getter.Alerts(childCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to query Prometheus alerts: %w", err)
	}

	return result.Alerts, nil
}

func hasFiringAlert(alerts []promv1.Alert, alertName string) bool {
	for _, alert := range alerts {
		if alert.State == promv1.AlertStateFiring &&
			hasMatchingLabelValue(alert, "alertname", alertName) {
			return true
		}
	}
	return false
}

func hasFiringExporterAlert(alerts []promv1.Alert, alertName, pipelineName string) bool {
	for _, alert := range alerts {
		if alert.State == promv1.AlertStateFiring &&
			hasMatchingLabelValue(alert, "alertname", alertName) &&
			hasMatchingLabelValue(alert, "exporter", pipelineName) {
			return true
		}
	}
	return false
}

func hasMatchingLabelValue(alert promv1.Alert, labelName, labelValue string) bool {
	v, ok := alert.Labels[model.LabelName(labelName)]
	return ok && strings.Contains(string(v), labelValue)
}
