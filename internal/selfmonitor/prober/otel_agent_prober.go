package prober

import (
	"context"
	"fmt"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/config"
)

// OTelAgentProber is a prober for OTel Agent
type OTelAgentProber struct {
	getter  alertGetter
	matcher matcherFunc
}

type OTelAgentProbeResult struct {
	PipelineProbeResult

	QueueAlmostFull bool
}

func NewOTelLogAgentProber(selfMonitorName types.NamespacedName) (*OTelAgentProber, error) {
	return newOTelAgentProber(selfMonitorName, config.MatchesLogPipelineRule)
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
	logf.FromContext(ctx).V(1).Info("retrieving alerts for OTel Agent prober", "pipeline", pipelineName)
	alerts, err := retrieveAlerts(ctx, p.getter)
	if err != nil {
		return OTelAgentProbeResult{}, fmt.Errorf("failed to retrieve alerts: %w", err)
	}
	logf.FromContext(ctx).V(1).Info("found alerts", "alerts", alerts)

	allDropped := p.isFiring(alerts, config.RuleNameAgentAllDataDropped, pipelineName)
	someDropped := p.isFiring(alerts, config.RuleNameAgentSomeDataDropped, pipelineName)
	queueAlmostFull := p.isFiring(alerts, config.RuleNameAgentQueueAlmostFull, pipelineName)
	healthy := !allDropped && !someDropped && !queueAlmostFull

	return OTelAgentProbeResult{
		PipelineProbeResult: PipelineProbeResult{
			AllDataDropped:  allDropped,
			SomeDataDropped: someDropped,
			Healthy:         healthy,
		},
		QueueAlmostFull: queueAlmostFull,
	}, nil
}

func (p *OTelAgentProber) isFiring(alerts []promv1.Alert, ruleName, pipelineName string) bool {
	return isFiringWithMatcher(alerts, ruleName, pipelineName, p.matcher)
}
