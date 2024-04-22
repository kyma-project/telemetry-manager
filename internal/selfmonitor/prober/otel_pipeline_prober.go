package prober

import (
	"context"
	"fmt"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/config"
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
	return newOTelPipelineProber(selfMonitorName, config.MatchesMetricPipelineRule)
}

func NewTracePipelineProber(selfMonitorName types.NamespacedName) (*OTelPipelineProber, error) {
	return newOTelPipelineProber(selfMonitorName, config.MatchesTracePipelineRule)
}

func newOTelPipelineProber(selfMonitorName types.NamespacedName, matcher matcherFunc) (*OTelPipelineProber, error) {
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
	exporterSentData := p.isFiring(alerts, config.RuleNameGatewayExporterSentData, pipelineName)
	exporterDroppedData := p.isFiring(alerts, config.RuleNameGatewayExporterDroppedData, pipelineName)
	exporterEnqueueFailed := p.isFiring(alerts, config.RuleNameGatewayExporterEnqueueFailed, pipelineName)

	return !exporterSentData && (exporterDroppedData || exporterEnqueueFailed)
}

func (p *OTelPipelineProber) someDataDropped(alerts []promv1.Alert, pipelineName string) bool {
	exporterSentData := p.isFiring(alerts, config.RuleNameGatewayExporterSentData, pipelineName)
	exporterDroppedData := p.isFiring(alerts, config.RuleNameGatewayExporterDroppedData, pipelineName)
	exporterEnqueueFailed := p.isFiring(alerts, config.RuleNameGatewayExporterEnqueueFailed, pipelineName)

	return exporterSentData && (exporterDroppedData || exporterEnqueueFailed)
}

func (p *OTelPipelineProber) queueAlmostFull(alerts []promv1.Alert, pipelineName string) bool {
	return p.isFiring(alerts, config.RuleNameGatewayExporterQueueAlmostFull, pipelineName)
}

func (p *OTelPipelineProber) throttling(alerts []promv1.Alert, pipelineName string) bool {
	return p.isFiring(alerts, config.RuleNameGatewayReceiverRefusedData, pipelineName)
}

func (p *OTelPipelineProber) healthy(alerts []promv1.Alert, pipelineName string) bool {
	return !(p.isFiring(alerts, config.RuleNameGatewayExporterDroppedData, pipelineName) ||
		p.isFiring(alerts, config.RuleNameGatewayExporterQueueAlmostFull, pipelineName) ||
		p.isFiring(alerts, config.RuleNameGatewayExporterEnqueueFailed, pipelineName) ||
		p.isFiring(alerts, config.RuleNameGatewayReceiverRefusedData, pipelineName))
}

func (p *OTelPipelineProber) isFiring(alerts []promv1.Alert, ruleName, pipelineName string) bool {
	return isFiringWithMatcher(alerts, ruleName, pipelineName, p.matcher)
}
