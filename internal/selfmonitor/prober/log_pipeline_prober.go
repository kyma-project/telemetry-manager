package prober

import (
	"context"
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/alertrules"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"k8s.io/apimachinery/pkg/types"
	"time"
)

type LogPipelineProber struct {
	clientTimeout time.Duration
	getter        alertGetter
	pipelineType  alertrules.PipelineType
}

type LogPipelineProbeResult struct {
	AllDataDropped  bool
	SomeDataDropped bool
	NoLogsDelivered bool
	BufferFillingUp bool
	Healthy         bool
}

func NewLogPipelineProber(selfMonitorName types.NamespacedName) (*LogPipelineProber, error) {
	return &LogPipelineProber{
		clientTimeout: clientTimeout,
	}, nil
}

func (p *LogPipelineProber) Probe(ctx context.Context, pipelineName string) (LogPipelineProbeResult, error) {
	alerts, err := p.retrieveAlerts(ctx)
	if err != nil {
		return LogPipelineProbeResult{}, fmt.Errorf("failed to retrieve alerts: %w", err)
	}

	return LogPipelineProbeResult{
		AllDataDropped:  p.allDataDropped(alerts, pipelineName),
		SomeDataDropped: p.someDataDropped(alerts, pipelineName),
		NoLogsDelivered: p.noLogsDelivered(alerts, pipelineName),
		BufferFillingUp: p.bufferFillingUp(alerts),
		Healthy:         p.healthy(alerts, pipelineName),
	}, nil
}

func (p *LogPipelineProber) retrieveAlerts(ctx context.Context) ([]promv1.Alert, error) {
	childCtx, cancel := context.WithTimeout(ctx, p.clientTimeout)
	defer cancel()

	result, err := p.getter.Alerts(childCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to query Prometheus alerts: %w", err)
	}

	return result.Alerts, nil
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
