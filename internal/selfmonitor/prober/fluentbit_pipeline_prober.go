package prober

import (
	"context"
	"fmt"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/config"
)

type FluentBitLogPipelineProber struct {
	getter alertGetter
}

type FluentBitLogPipelineProbeResult struct {
	PipelineProbeResult

	NoLogsDelivered bool
	BufferFillingUp bool
}

func NewFluentBitLogPipelineProber(selfMonitorName types.NamespacedName) (*FluentBitLogPipelineProber, error) {
	promClient, err := newPrometheusClient(selfMonitorName)
	if err != nil {
		return nil, err
	}

	return &FluentBitLogPipelineProber{
		getter: promClient,
	}, nil
}

//nolint:dupl // Keep it duplicated for now, as Fluent Bit logging will be replaced by OpenTelemetry
func (p *FluentBitLogPipelineProber) Probe(ctx context.Context, pipelineName string) (FluentBitLogPipelineProbeResult, error) {
	alerts, err := retrieveAlerts(ctx, p.getter)
	if err != nil {
		return FluentBitLogPipelineProbeResult{}, fmt.Errorf("failed to retrieve alerts: %w", err)
	}

	allDropped := p.isFiring(alerts, config.RuleNameLogFluentBitAllDataDropped, pipelineName)
	someDropped := p.isFiring(alerts, config.RuleNameLogFluentBitSomeDataDropped, pipelineName)
	bufferFillingUp := p.isFiring(alerts, config.RuleNameLogFluentBitBufferInUse, pipelineName)
	noLogs := p.isFiring(alerts, config.RuleNameLogFluentBitNoLogsDelivered, pipelineName)
	healthy := !allDropped && !someDropped && !bufferFillingUp && !noLogs

	return FluentBitLogPipelineProbeResult{
		PipelineProbeResult: PipelineProbeResult{
			AllDataDropped:  allDropped,
			SomeDataDropped: someDropped,
			Healthy:         healthy,
		},
		NoLogsDelivered: noLogs,
		BufferFillingUp: bufferFillingUp,
	}, nil
}

func (p *FluentBitLogPipelineProber) isFiring(alerts []promv1.Alert, ruleName, pipelineName string) bool {
	return isFiringWithMatcher(alerts, ruleName, pipelineName, config.MatchesFluentBitLogPipelineRule)
}
