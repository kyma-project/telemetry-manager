package prober

import (
	"context"
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/alertrules"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"k8s.io/apimachinery/pkg/types"
)

type LogPipelineProber struct {
	getter       alertGetter
	pipelineType alertrules.PipelineType
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
		BufferFillingUp: p.bufferFillingUp(alerts),
	}, nil
}

func (p *LogPipelineProber) allDataDropped(alerts []promv1.Alert, pipelineName string) bool {
	exporterSentLogs := p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameLogAgentExporterSentLogs, pipelineName)
	bufferFull := p.hasFiringAlert(alerts, alertrules.RuleNameLogAgentBufferFull)
	return !exporterSentLogs && bufferFull
}

func (p *LogPipelineProber) someDataDropped(alerts []promv1.Alert, pipelineName string) bool {
	exporterSentLogs := p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameLogAgentExporterSentLogs, pipelineName)
	bufferFull := p.hasFiringAlert(alerts, alertrules.RuleNameLogAgentBufferFull)
	return exporterSentLogs && bufferFull
}

func (p *LogPipelineProber) noLogsDelivered(alerts []promv1.Alert, pipelineName string) bool {
	exporterSentLogs := p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameLogAgentExporterSentLogs, pipelineName)
	receiverReadLogs := p.hasFiringAlertForPipeline(alerts, alertrules.RuleNameLogAgentReceiverReadLogs, pipelineName)
	return exporterSentLogs && receiverReadLogs
}

func (p *LogPipelineProber) bufferFillingUp(alerts []promv1.Alert) bool {
	return p.hasFiringAlert(alerts, alertrules.RuleNameLogAgentBufferInUse)
}

func (p *LogPipelineProber) healthy(alerts []promv1.Alert, pipelineName string) bool {
	bufferFull := p.hasFiringAlert(alerts, alertrules.RuleNameLogAgentBufferFull)
	bufferInUse := p.hasFiringAlert(alerts, alertrules.RuleNameLogAgentBufferInUse)
	return !bufferFull && !bufferInUse
}

func (p *LogPipelineProber) hasFiringAlert(alerts []promv1.Alert, alertName string) bool {
	for _, alert := range alerts {
		if alert.State == promv1.AlertStateFiring &&
			p.matchesAlertName(alert, alertName) {
			return true
		}
	}
	return false
}

func (p *LogPipelineProber) hasFiringAlertForPipeline(alerts []promv1.Alert, alertName, pipelineName string) bool {
	for _, alert := range alerts {
		if alert.State == promv1.AlertStateFiring &&
			p.matchesAlertName(alert, alertName) &&
			p.matchesPipeline(alert, pipelineName) {
			return true
		}
	}
	return false
}

func (p *LogPipelineProber) matchesAlertName(alert promv1.Alert, alertName string) bool {
	v, ok := alert.Labels[model.AlertNameLabel]
	return ok && string(v) == alertName
}

func (p *LogPipelineProber) matchesPipeline(alert promv1.Alert, pipelineName string) bool {
	labelValue, ok := alert.Labels[model.LabelName("name")]
	if !ok {
		return false
	}

	return string(labelValue) == pipelineName
}
