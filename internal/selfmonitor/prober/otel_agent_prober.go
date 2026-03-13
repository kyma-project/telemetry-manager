package prober

import (
	"context"
	"fmt"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"k8s.io/apimachinery/pkg/types"

	selfmonitorconfig "github.com/kyma-project/telemetry-manager/internal/selfmonitor/config"
)

// OTelAgentProber is a prober for OTel Agent
type OTelAgentProber struct {
	getter  alertGetter
	matcher matcherFunc
}

type OTelAgentProbeResult struct {
	PipelineProbeResult
}

func NewOTelLogAgentProber(selfMonitorName types.NamespacedName) (*OTelAgentProber, error) {
	return newOTelAgentProber(selfMonitorName, selfmonitorconfig.MatchesLogPipelineRule)
}

func newOTelAgentProber(selfMonitorName types.NamespacedName, matcher matcherFunc) (*OTelAgentProber, error) {
	promClient, err := newPrometheusClient(selfMonitorName)
	if err != nil {
		return nil, err
	}

	return &OTelAgentProber{
		getter:  promClient,
		matcher: matcher,
	}, nil
}

func (p *OTelAgentProber) Probe(ctx context.Context, pipelineName string) (OTelAgentProbeResult, error) {
	alerts, err := retrieveAlerts(ctx, p.getter)
	if err != nil {
		return OTelAgentProbeResult{}, fmt.Errorf("failed to retrieve alerts: %w", err)
	}

	allDropped := p.isFiring(alerts, selfmonitorconfig.RuleNameAgentAllDataDropped, pipelineName)
	someDropped := p.isFiring(alerts, selfmonitorconfig.RuleNameAgentSomeDataDropped, pipelineName)
	healthy := !allDropped && !someDropped

	return OTelAgentProbeResult{
		PipelineProbeResult: PipelineProbeResult{
			AllDataDropped:  allDropped,
			SomeDataDropped: someDropped,
			Healthy:         healthy,
		},
	}, nil
}

func (p *OTelAgentProber) isFiring(alerts []promv1.Alert, ruleName, pipelineName string) bool {
	return isFiringWithMatcher(alerts, ruleName, pipelineName, p.matcher)
}
