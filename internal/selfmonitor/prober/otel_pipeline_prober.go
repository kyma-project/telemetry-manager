package prober

import (
	"context"
	"fmt"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/alertrules"
)

// OTelPipelineProber is a prober for OTel Collector pipelines
type OTelPipelineProber struct {
	getter alertGetter
}

type OTelPipelineProbeResult struct {
	PipelineProbeResult

	QueueAlmostFull bool
	Throttling      bool
}

func NewMetricPipelineProber(selfMonitorName types.NamespacedName) (*OTelPipelineProber, error) {
	return newOTelPipelineProber(selfMonitorName)
}

func NewTracePipelineProber(selfMonitorName types.NamespacedName) (*OTelPipelineProber, error) {
	return newOTelPipelineProber(selfMonitorName)
}

func newOTelPipelineProber(selfMonitorName types.NamespacedName) (*OTelPipelineProber, error) {
	promClient, err := newPrometheusClient(selfMonitorName)
	if err != nil {
		return nil, err
	}

	return &OTelPipelineProber{
		getter: promClient,
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
		Throttling:      p.throttling(alerts, pipelineName),
	}, nil
}

func (p *OTelPipelineProber) allDataDropped(alerts []promv1.Alert, pipelineName string) bool {
	exporterSentFiring := evaluateRule(alerts, alertrules.RuleNameGatewayExporterSentData, pipelineName)
	exporterDroppedFiring := evaluateRule(alerts, alertrules.RuleNameGatewayExporterDroppedData, pipelineName)
	exporterEnqueueFailedFiring := evaluateRule(alerts, alertrules.RuleNameGatewayExporterEnqueueFailed, pipelineName)

	return !exporterSentFiring && (exporterDroppedFiring || exporterEnqueueFailedFiring)
}

func (p *OTelPipelineProber) someDataDropped(alerts []promv1.Alert, pipelineName string) bool {
	exporterSentFiring := evaluateRule(alerts, alertrules.RuleNameGatewayExporterSentData, pipelineName)
	exporterDroppedFiring := evaluateRule(alerts, alertrules.RuleNameGatewayExporterDroppedData, pipelineName)
	exporterEnqueueFailedFiring := evaluateRule(alerts, alertrules.RuleNameGatewayExporterEnqueueFailed, pipelineName)

	return exporterSentFiring && (exporterDroppedFiring || exporterEnqueueFailedFiring)
}

func (p *OTelPipelineProber) queueAlmostFull(alerts []promv1.Alert, pipelineName string) bool {
	return evaluateRule(alerts, alertrules.RuleNameGatewayExporterQueueAlmostFull, pipelineName)
}

func (p *OTelPipelineProber) throttling(alerts []promv1.Alert, pipelineName string) bool {
	return evaluateRule(alerts, alertrules.RuleNameGatewayReceiverRefusedData, pipelineName)
}

func (p *OTelPipelineProber) healthy(alerts []promv1.Alert, pipelineName string) bool {
	return !(evaluateRule(alerts, alertrules.RuleNameGatewayExporterDroppedData, pipelineName) ||
		evaluateRule(alerts, alertrules.RuleNameGatewayExporterQueueAlmostFull, pipelineName) ||
		evaluateRule(alerts, alertrules.RuleNameGatewayExporterEnqueueFailed, pipelineName) ||
		evaluateRule(alerts, alertrules.RuleNameGatewayReceiverRefusedData, pipelineName))
}
