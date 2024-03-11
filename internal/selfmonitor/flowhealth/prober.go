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
	AllDataDropped    bool
	SomeDataDropped   bool
	QueueAlmostFull   bool
	GatewayThrottling bool
	Healthy           bool
}

func (p *Prober) Probe(ctx context.Context, pipelineName string) (ProbeResult, error) {
	allDataDropped, err := p.allDataDropped(ctx, pipelineName)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("failed to probe all data dropped: %w", err)
	}

	someDataDropped, err := p.someDataDropped(ctx, pipelineName)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("failed to probe some data dropped: %w", err)
	}

	queueAlmostFull, err := p.queueAlmostFull(ctx, pipelineName)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("failed to probe buffer filling up: %w", err)
	}

	return ProbeResult{
		AllDataDropped:  allDataDropped,
		SomeDataDropped: someDataDropped,
		QueueAlmostFull: queueAlmostFull,
	}, nil
}

func (p *Prober) allDataDropped(ctx context.Context, pipelineName string) (bool, error) {
	alerts, err := p.retrieveAlerts(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to retrieve alerts: %w", err)
	}

	exporterSentFiring := hasFiringAlert(alerts, alertNameExporterSentData, pipelineName)
	exporterDroppedFiring := hasFiringAlert(alerts, alertNameExporterDroppedData, pipelineName)
	exporterEnqueueFailedFiring := hasFiringAlert(alerts, alertNameExporterEnqueueFailed, pipelineName)

	return !exporterSentFiring && (exporterDroppedFiring || exporterEnqueueFailedFiring), nil
}

func (p *Prober) someDataDropped(ctx context.Context, pipelineName string) (bool, error) {
	alerts, err := p.retrieveAlerts(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to retrieve alerts: %w", err)
	}

	exporterSentFiring := hasFiringAlert(alerts, alertNameExporterSentData, pipelineName)
	exporterDroppedFiring := hasFiringAlert(alerts, alertNameExporterDroppedData, pipelineName)
	exporterEnqueueFailedFiring := hasFiringAlert(alerts, alertNameExporterEnqueueFailed, pipelineName)

	return exporterSentFiring && (exporterDroppedFiring || exporterEnqueueFailedFiring), nil
}

func (p *Prober) queueAlmostFull(ctx context.Context, pipelineName string) (bool, error) {
	alerts, err := p.retrieveAlerts(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to retrieve alerts: %w", err)
	}

	return hasFiringAlert(alerts, alertNameExporterQueueAlmostFull, pipelineName), nil
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

func hasFiringAlert(alerts []promv1.Alert, alertName, pipelineName string) bool {
	for _, alert := range alerts {
		isFiring := alert.State == promv1.AlertStateFiring
		hasMatchingName := hasMatchingLabelValue(alert, "alertname", alertName)
		hasMatchingExporter := hasMatchingLabelValue(alert, "exporter", pipelineName)

		if isFiring && hasMatchingName && hasMatchingExporter {
			return true
		}
	}

	return false
}

func hasMatchingLabelValue(alert promv1.Alert, labelName, labelValue string) bool {
	v, ok := alert.Labels[model.LabelName(labelName)]
	return ok && strings.Contains(string(v), labelValue)
}
