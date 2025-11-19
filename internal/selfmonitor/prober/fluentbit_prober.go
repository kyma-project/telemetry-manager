package prober

import (
	"context"
	"fmt"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"k8s.io/apimachinery/pkg/types"

	selfmonitorconfig "github.com/kyma-project/telemetry-manager/internal/selfmonitor/config"
)

type FluentBitProber struct {
	getter alertGetter
}

type FluentBitProbeResult struct {
	PipelineProbeResult

	NoLogsDelivered bool
	BufferFillingUp bool
}

func NewFluentBitProber(selfMonitorName types.NamespacedName) (*FluentBitProber, error) {
	promClient, err := newPrometheusClient(selfMonitorName)
	if err != nil {
		return nil, err
	}

	return &FluentBitProber{
		getter: promClient,
	}, nil
}

//nolint:dupl // Keep it duplicated for now, as Fluent Bit logging will be replaced by OpenTelemetry
func (p *FluentBitProber) Probe(ctx context.Context, pipelineName string) (FluentBitProbeResult, error) {
	alerts, err := retrieveAlerts(ctx, p.getter)
	if err != nil {
		return FluentBitProbeResult{}, fmt.Errorf("failed to retrieve alerts: %w", err)
	}

	allDropped := p.isFiring(alerts, selfmonitorconfig.RuleNameLogFluentBitAllDataDropped, pipelineName)
	someDropped := p.isFiring(alerts, selfmonitorconfig.RuleNameLogFluentBitSomeDataDropped, pipelineName)
	bufferFillingUp := p.isFiring(alerts, selfmonitorconfig.RuleNameLogFluentBitBufferInUse, pipelineName)
	noLogs := p.isFiring(alerts, selfmonitorconfig.RuleNameLogFluentBitNoLogsDelivered, pipelineName)
	healthy := !allDropped && !someDropped && !bufferFillingUp && !noLogs

	return FluentBitProbeResult{
		PipelineProbeResult: PipelineProbeResult{
			AllDataDropped:  allDropped,
			SomeDataDropped: someDropped,
			Healthy:         healthy,
		},
		NoLogsDelivered: noLogs,
		BufferFillingUp: bufferFillingUp,
	}, nil
}

func (p *FluentBitProber) isFiring(alerts []promv1.Alert, ruleName, pipelineName string) bool {
	return isFiringWithMatcher(alerts, ruleName, pipelineName, selfmonitorconfig.MatchesLogPipelineRule)
}
