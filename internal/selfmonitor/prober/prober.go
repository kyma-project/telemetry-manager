package prober

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/alertrules"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/ports"
)

const (
	clientTimeout = 10 * time.Second
)

//go:generate mockery --name alertGetter --filename=alert_getter.go --exported
type alertGetter interface {
	Alerts(ctx context.Context) (promv1.AlertsResult, error)
}

type Prober struct {
	clientTimeout time.Duration
	getter        alertGetter
	pipelineType  alertrules.PipelineType
}

func NewProber(pipelineType alertrules.PipelineType, selfMonitorName types.NamespacedName) (*Prober, error) {
	client, err := api.NewClient(api.Config{
		Address: fmt.Sprintf("http://%s.%s:%d", selfMonitorName.Name, selfMonitorName.Namespace, ports.PrometheusPort),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus client: %w", err)
	}

	return &Prober{
		getter:        promv1.NewAPI(client),
		clientTimeout: clientTimeout,
		pipelineType:  pipelineType,
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
		AllDataDropped:  p.allDataDropped(alerts, pipelineName),
		SomeDataDropped: p.someDataDropped(alerts, pipelineName),
		QueueAlmostFull: p.queueAlmostFull(alerts, pipelineName),
		Throttling:      p.throttling(alerts),
		Healthy:         p.healthy(alerts, pipelineName),
	}, nil
}

func (p *Prober) allDataDropped(alerts []promv1.Alert, pipelineName string) bool {
	exporterSentFiring := p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameGatewayExporterSentData, pipelineName)
	exporterDroppedFiring := p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameGatewayExporterDroppedData, pipelineName)
	exporterEnqueueFailedFiring := p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameGatewayExporterEnqueueFailed, pipelineName)

	return !exporterSentFiring && (exporterDroppedFiring || exporterEnqueueFailedFiring)
}

func (p *Prober) someDataDropped(alerts []promv1.Alert, pipelineName string) bool {
	exporterSentFiring := p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameGatewayExporterSentData, pipelineName)
	exporterDroppedFiring := p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameGatewayExporterDroppedData, pipelineName)
	exporterEnqueueFailedFiring := p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameGatewayExporterEnqueueFailed, pipelineName)

	return exporterSentFiring && (exporterDroppedFiring || exporterEnqueueFailedFiring)
}

func (p *Prober) queueAlmostFull(alerts []promv1.Alert, pipelineName string) bool {
	return p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameGatewayExporterQueueAlmostFull, pipelineName)
}

func (p *Prober) throttling(alerts []promv1.Alert) bool {
	return p.hasFiringAlert(alerts, alertrules.RuleNameGatewayReceiverRefusedData)
}

func (p *Prober) healthy(alerts []promv1.Alert, pipelineName string) bool {
	return !(p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameGatewayExporterDroppedData, pipelineName) ||
		p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameGatewayExporterQueueAlmostFull, pipelineName) ||
		p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameGatewayExporterEnqueueFailed, pipelineName) ||
		p.hasFiringAlert(alerts, alertrules.RuleNameGatewayReceiverRefusedData))
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

func (p *Prober) hasFiringAlert(alerts []promv1.Alert, alertName string) bool {
	for _, alert := range alerts {
		if alert.State == promv1.AlertStateFiring &&
			p.matchesAlertName(alert, alertName) {
			return true
		}
	}
	return false
}

func (p *Prober) hasFiringAlertForPipeline(alerts []promv1.Alert, alertName, pipelineName string) bool {
	for _, alert := range alerts {
		if alert.State == promv1.AlertStateFiring &&
			p.matchesAlertName(alert, alertName) &&
			matchesPipeline(alert, pipelineName) {
			return true
		}
	}
	return false
}

func (p *Prober) matchesAlertName(alert promv1.Alert, alertName string) bool {
	v, ok := alert.Labels[model.AlertNameLabel]
	expectedFullName := alertrules.RuleNamePrefix(p.pipelineType) + alertName
	return ok && string(v) == expectedFullName
}

func matchesPipeline(alert promv1.Alert, pipelineName string) bool {
	labelValue, ok := alert.Labels[model.LabelName(alertrules.LabelExporter)]
	if !ok {
		return false
	}

	exportedID := string(labelValue)
	return otlpexporter.ExporterID(telemetryv1alpha1.OtlpProtocolHTTP, pipelineName) == exportedID || otlpexporter.ExporterID(telemetryv1alpha1.OtlpProtocolGRPC, pipelineName) == exportedID
}
