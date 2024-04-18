package prober

import (
	"context"
	"fmt"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/config"
)

type LogPipelineProber struct {
	getter alertGetter
}

type LogPipelineProbeResult struct {
	PipelineProbeResult

	NoLogsDelivered bool
	BufferFillingUp bool
}

func NewLogPipelineProber(selfMonitorName types.NamespacedName) (*LogPipelineProber, error) {
	promClient, err := newPrometheusClient(selfMonitorName)
	if err != nil {
		return nil, err
	}

	return &LogPipelineProber{
		getter: promClient,
	}, nil
}

func (p *LogPipelineProber) Probe(ctx context.Context, pipelineName string) (LogPipelineProbeResult, error) {
	alerts, err := retrieveAlerts(ctx, p.getter)
	if err != nil {
		return LogPipelineProbeResult{}, fmt.Errorf("failed to retrieve alerts: %w", err)
	}

	return LogPipelineProbeResult{
		PipelineProbeResult: PipelineProbeResult{
			AllDataDropped:  p.allDataDropped(alerts, pipelineName),
			SomeDataDropped: p.someDataDropped(alerts, pipelineName),
			Healthy:         p.healthy(alerts, pipelineName),
		},
		NoLogsDelivered: p.noLogsDelivered(alerts, pipelineName),
		BufferFillingUp: p.bufferFillingUp(alerts, pipelineName),
	}, nil
}

func (p *LogPipelineProber) allDataDropped(alerts []promv1.Alert, pipelineName string) bool {
	exporterSentLogs := p.evaluateRule(alerts, config.RuleNameLogAgentExporterSentLogs, pipelineName)
	exporterDroppedLogs := p.evaluateRule(alerts, config.RuleNameLogAgentExporterDroppedLogs, pipelineName)
	bufferFull := p.evaluateRule(alerts, config.RuleNameLogAgentBufferFull, pipelineName)
	return !exporterSentLogs && (exporterDroppedLogs || bufferFull)
}

func (p *LogPipelineProber) someDataDropped(alerts []promv1.Alert, pipelineName string) bool {
	exporterSentLogs := p.evaluateRule(alerts, config.RuleNameLogAgentExporterSentLogs, pipelineName)
	exporterDroppedLogs := p.evaluateRule(alerts, config.RuleNameLogAgentExporterDroppedLogs, pipelineName)
	bufferFull := p.evaluateRule(alerts, config.RuleNameLogAgentBufferFull, pipelineName)
	return exporterSentLogs && (exporterDroppedLogs || bufferFull)
}

func (p *LogPipelineProber) noLogsDelivered(alerts []promv1.Alert, pipelineName string) bool {
	receiverReadLogs := p.evaluateRule(alerts, config.RuleNameLogAgentReceiverReadLogs, pipelineName)
	exporterSentLogs := p.evaluateRule(alerts, config.RuleNameLogAgentExporterSentLogs, pipelineName)
	return receiverReadLogs && !exporterSentLogs
}

func (p *LogPipelineProber) bufferFillingUp(alerts []promv1.Alert, pipelineName string) bool {
	return p.evaluateRule(alerts, config.RuleNameLogAgentBufferInUse, pipelineName)
}

func (p *LogPipelineProber) healthy(alerts []promv1.Alert, pipelineName string) bool {
	// The pipeline is healthy if none of the following conditions are met:
	bufferInUse := p.evaluateRule(alerts, config.RuleNameLogAgentBufferInUse, pipelineName)
	bufferFull := p.evaluateRule(alerts, config.RuleNameLogAgentBufferFull, pipelineName)
	exporterDroppedLogs := p.evaluateRule(alerts, config.RuleNameLogAgentExporterDroppedLogs, pipelineName)

	// The pipeline is healthy if either no logs are being read or all logs are being sent
	receiverReadLogs := p.evaluateRule(alerts, config.RuleNameLogAgentReceiverReadLogs, pipelineName)
	exporterSentLogs := p.evaluateRule(alerts, config.RuleNameLogAgentExporterSentLogs, pipelineName)
	return !(bufferInUse || bufferFull || exporterDroppedLogs) && (!receiverReadLogs || exporterSentLogs)
}

func (p *LogPipelineProber) evaluateRule(alerts []promv1.Alert, alertName, pipelineName string) bool {
	return evaluateRuleWithMatcher(alerts, alertName, pipelineName, config.MatchesLogPipelineRule)
}
