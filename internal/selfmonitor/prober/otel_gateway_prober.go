package prober

import (
	"context"
	"fmt"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/config"
)

// OTelGatewayProber is a prober for OTel Gateway
type OTelGatewayProber struct {
	getter  alertGetter
	matcher matcherFunc
}

type OTelGatewayProbeResult struct {
	PipelineProbeResult

	Throttling bool
}

func NewOTelMetricGatewayProber(selfMonitorName types.NamespacedName) (*OTelGatewayProber, error) {
	return newOTelGatewayProber(selfMonitorName, config.MatchesMetricPipelineRule)
}

func NewOTelMetricAgentProber(selfMonitorName types.NamespacedName) (*OTelAgentProber, error) {
	return newOTelAgentProber(selfMonitorName, config.MatchesMetricPipelineRule)
}

func NewOTelTraceGatewayProber(selfMonitorName types.NamespacedName) (*OTelGatewayProber, error) {
	return newOTelGatewayProber(selfMonitorName, config.MatchesTracePipelineRule)
}

func NewOTelLogGatewayProber(selfMonitorName types.NamespacedName) (*OTelGatewayProber, error) {
	return newOTelGatewayProber(selfMonitorName, config.MatchesLogPipelineRule)
}

func newOTelGatewayProber(selfMonitorName types.NamespacedName, matcher matcherFunc) (*OTelGatewayProber, error) {
	promClient, err := newPrometheusClient(selfMonitorName)
	if err != nil {
		return nil, err
	}

	return &OTelGatewayProber{
		getter:  promClient,
		matcher: matcher,
	}, nil
}

//nolint:dupl // Keep it duplicated for now, as Fluent Bit logging will be replaced by OpenTelemetry
func (p *OTelGatewayProber) Probe(ctx context.Context, pipelineName string) (OTelGatewayProbeResult, error) {
	alerts, err := retrieveAlerts(ctx, p.getter)
	if err != nil {
		return OTelGatewayProbeResult{}, fmt.Errorf("failed to retrieve alerts: %w", err)
	}

	allDropped := p.isFiring(alerts, config.RuleNameGatewayAllDataDropped, pipelineName)
	someDropped := p.isFiring(alerts, config.RuleNameGatewaySomeDataDropped, pipelineName)
	throttling := p.isFiring(alerts, config.RuleNameGatewayThrottling, pipelineName)
	healthy := !allDropped && !someDropped && !throttling

	return OTelGatewayProbeResult{
		PipelineProbeResult: PipelineProbeResult{
			AllDataDropped:  allDropped,
			SomeDataDropped: someDropped,
			Healthy:         healthy,
		},
		Throttling: throttling,
	}, nil
}

func (p *OTelGatewayProber) isFiring(alerts []promv1.Alert, ruleName, pipelineName string) bool {
	return isFiringWithMatcher(alerts, ruleName, pipelineName, p.matcher)
}
