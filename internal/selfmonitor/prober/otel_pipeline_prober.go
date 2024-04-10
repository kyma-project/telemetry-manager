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
	getter  alertGetter
	matcher matcherFunc
}

type OTelPipelineProbeResult struct {
	PipelineProbeResult

	QueueAlmostFull bool
	Throttling      bool
}

func NewMetricPipelineProber(selfMonitorName types.NamespacedName) (*OTelPipelineProber, error) {
	return newOTelPipelineProber(alertrules.MetricPipeline, selfMonitorName, alertrules.MatchesMetricPipelineRule)
}

func NewTracePipelineProber(selfMonitorName types.NamespacedName) (*OTelPipelineProber, error) {
	return newOTelPipelineProber(alertrules.TracePipeline, selfMonitorName, alertrules.MatchesTracePipelineRule)
}

func newOTelPipelineProber(pipelineType alertrules.PipelineType, selfMonitorName types.NamespacedName, matcher matcherFunc) (*OTelPipelineProber, error) {
	promClient, err := newPrometheusClient(selfMonitorName)
	if err != nil {
		return nil, err
	}

	return &OTelPipelineProber{
		getter:  promClient,
		matcher: matcher,
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
	exporterSentFiring := p.evaluateRule(alerts, alertrules.RuleNameGatewayExporterSentData, pipelineName)
	exporterDroppedFiring := p.evaluateRule(alerts, alertrules.RuleNameGatewayExporterDroppedData, pipelineName)
	exporterEnqueueFailedFiring := p.evaluateRule(alerts, alertrules.RuleNameGatewayExporterEnqueueFailed, pipelineName)

	return !exporterSentFiring && (exporterDroppedFiring || exporterEnqueueFailedFiring)
}

func (p *OTelPipelineProber) someDataDropped(alerts []promv1.Alert, pipelineName string) bool {
	exporterSentFiring := p.evaluateRule(alerts, alertrules.RuleNameGatewayExporterSentData, pipelineName)
	exporterDroppedFiring := p.evaluateRule(alerts, alertrules.RuleNameGatewayExporterDroppedData, pipelineName)
	exporterEnqueueFailedFiring := p.evaluateRule(alerts, alertrules.RuleNameGatewayExporterEnqueueFailed, pipelineName)

	return exporterSentFiring && (exporterDroppedFiring || exporterEnqueueFailedFiring)
}

func (p *OTelPipelineProber) queueAlmostFull(alerts []promv1.Alert, pipelineName string) bool {
	return p.evaluateRule(alerts, alertrules.RuleNameGatewayExporterQueueAlmostFull, pipelineName)
}

func (p *OTelPipelineProber) throttling(alerts []promv1.Alert, pipelineName string) bool {
	return p.evaluateRule(alerts, alertrules.RuleNameGatewayReceiverRefusedData, pipelineName)
}

func (p *OTelPipelineProber) healthy(alerts []promv1.Alert, pipelineName string) bool {
	return !(p.evaluateRule(alerts, alertrules.RuleNameGatewayExporterDroppedData, pipelineName) ||
		p.evaluateRule(alerts, alertrules.RuleNameGatewayExporterQueueAlmostFull, pipelineName) ||
		p.evaluateRule(alerts, alertrules.RuleNameGatewayExporterEnqueueFailed, pipelineName) ||
		p.evaluateRule(alerts, alertrules.RuleNameGatewayReceiverRefusedData, pipelineName))
}

func (p *OTelPipelineProber) evaluateRule(alerts []promv1.Alert, alertName, pipelineName string) bool {
	return evaluateRuleWithMatcher(alerts, alertName, pipelineName, p.matcher)
}
