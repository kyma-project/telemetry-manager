package prober

import (
	"context"
	"fmt"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/alertrules"
)

// OTelPipelineProber is a prober for OTel Collector pipelines
type OTelPipelineProber struct {
	getter       alertGetter
	pipelineType alertrules.PipelineType
}

type OTelPipelineProbeResult struct {
	PipelineProbeResult

	QueueAlmostFull bool
	Throttling      bool
}

func NewOTelPipelineProber(pipelineType alertrules.PipelineType, selfMonitorName types.NamespacedName) (*OTelPipelineProber, error) {
	promClient, err := newPrometheusClient(selfMonitorName)
	if err != nil {
		return nil, err
	}

	return &OTelPipelineProber{
		getter:       promClient,
		pipelineType: pipelineType,
	}, nil
}

func (p *OTelPipelineProber) Probe(ctx context.Context, pipelineName string) (OTelPipelineProbeResult, error) {
	alerts, err := retrieveAlerts(ctx, p.getter)
	if err != nil {
		return OTelPipelineProbeResult{}, fmt.Errorf("failed to retrieve alerts: %w", err)
	}

	return OTelPipelineProbeResult{
		PipelineProbeResult: PipelineProbeResult{
			AllDataDropped:  p.allDataDropped(alerts, pipelineName),
			SomeDataDropped: p.someDataDropped(alerts, pipelineName),
			Healthy:         p.healthy(alerts, pipelineName),
		},
		QueueAlmostFull: p.queueAlmostFull(alerts, pipelineName),
		Throttling:      p.throttling(alerts),
	}, nil
}

func (p *OTelPipelineProber) allDataDropped(alerts []promv1.Alert, pipelineName string) bool {
	exporterSentFiring := p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameGatewayExporterSentData, pipelineName)
	exporterDroppedFiring := p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameGatewayExporterDroppedData, pipelineName)
	exporterEnqueueFailedFiring := p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameGatewayExporterEnqueueFailed, pipelineName)

	return !exporterSentFiring && (exporterDroppedFiring || exporterEnqueueFailedFiring)
}

func (p *OTelPipelineProber) someDataDropped(alerts []promv1.Alert, pipelineName string) bool {
	exporterSentFiring := p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameGatewayExporterSentData, pipelineName)
	exporterDroppedFiring := p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameGatewayExporterDroppedData, pipelineName)
	exporterEnqueueFailedFiring := p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameGatewayExporterEnqueueFailed, pipelineName)

	return exporterSentFiring && (exporterDroppedFiring || exporterEnqueueFailedFiring)
}

func (p *OTelPipelineProber) queueAlmostFull(alerts []promv1.Alert, pipelineName string) bool {
	return p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameGatewayExporterQueueAlmostFull, pipelineName)
}

func (p *OTelPipelineProber) throttling(alerts []promv1.Alert) bool {
	return p.hasFiringAlert(alerts, alertrules.RuleNameGatewayReceiverRefusedData)
}

func (p *OTelPipelineProber) healthy(alerts []promv1.Alert, pipelineName string) bool {
	return !(p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameGatewayExporterDroppedData, pipelineName) ||
		p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameGatewayExporterQueueAlmostFull, pipelineName) ||
		p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameGatewayExporterEnqueueFailed, pipelineName) ||
		p.hasFiringAlert(alerts, alertrules.RuleNameGatewayReceiverRefusedData))
}

func (p *OTelPipelineProber) hasFiringAlert(alerts []promv1.Alert, alertName string) bool {
	for _, alert := range alerts {
		if alert.State == promv1.AlertStateFiring &&
			p.matchesAlertName(alert, alertName) {
			return true
		}
	}
	return false
}

func (p *OTelPipelineProber) hasFiringAlertForPipeline(alerts []promv1.Alert, alertName, pipelineName string) bool {
	for _, alert := range alerts {
		if alert.State == promv1.AlertStateFiring &&
			p.matchesAlertName(alert, alertName) &&
			p.matchesPipeline(alert, pipelineName) {
			return true
		}
	}
	return false
}

func (p *OTelPipelineProber) matchesAlertName(alert promv1.Alert, alertName string) bool {
	v, ok := alert.Labels[model.AlertNameLabel]
	expectedFullName := alertrules.RuleNamePrefix(p.pipelineType) + alertName
	return ok && string(v) == expectedFullName
}

func (p *OTelPipelineProber) matchesPipeline(alert promv1.Alert, pipelineName string) bool {
	labelValue, ok := alert.Labels[model.LabelName(alertrules.LabelExporter)]
	if !ok {
		return false
	}

	exportedID := string(labelValue)
	return otlpexporter.ExporterID(telemetryv1alpha1.OtlpProtocolHTTP, pipelineName) == exportedID || otlpexporter.ExporterID(telemetryv1alpha1.OtlpProtocolGRPC, pipelineName) == exportedID
}
