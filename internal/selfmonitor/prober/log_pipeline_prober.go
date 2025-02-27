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

func NewOtelLogPipelineProber(selfMonitorName types.NamespacedName) (*OTelPipelineProber, error) {
	return newOTelPipelineProber(selfMonitorName, config.MatchesLogPipelineRule)
}

func NewFluentBitLogPipelineProber(selfMonitorName types.NamespacedName) (*LogPipelineProber, error) {
	promClient, err := newPrometheusClient(selfMonitorName)
	if err != nil {
		return nil, err
	}

	return &LogPipelineProber{
		getter: promClient,
	}, nil
}

//nolint:dupl // Keep it duplicated for now, as Fluent Bit logging will be replaced by OpenTelemetry
func (p *LogPipelineProber) Probe(ctx context.Context, pipelineName string) (LogPipelineProbeResult, error) {
	alerts, err := retrieveAlerts(ctx, p.getter)
	if err != nil {
		return LogPipelineProbeResult{}, fmt.Errorf("failed to retrieve alerts: %w", err)
	}

	allDropped := p.isFiring(alerts, config.RuleNameAllDataDropped, pipelineName)
	someDropped := p.isFiring(alerts, config.RuleNameSomeDataDropped, pipelineName)
	bufferFillingUp := p.isFiring(alerts, config.RuleNameFluentBitBufferInUse, pipelineName)
	noLogs := p.isFiring(alerts, config.RuleNameFluentBitNoLogsDelivered, pipelineName)
	healthy := !(allDropped || someDropped || bufferFillingUp || noLogs)

	return LogPipelineProbeResult{
		PipelineProbeResult: PipelineProbeResult{
			AllDataDropped:  allDropped,
			SomeDataDropped: someDropped,
			Healthy:         healthy,
		},
		NoLogsDelivered: noLogs,
		BufferFillingUp: bufferFillingUp,
	}, nil
}

func (p *LogPipelineProber) isFiring(alerts []promv1.Alert, ruleName, pipelineName string) bool {
	return isFiringWithMatcher(alerts, ruleName, pipelineName, config.MatchesFluentBitLogPipelineRule)
}
