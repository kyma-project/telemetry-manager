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

//nolint:dupl // Keep it duplicated for now, as Fluent Bit logging will be replaced by OpenTelemetry
func (p *OTelPipelineProber) Probe(ctx context.Context, pipelineName string) (OTelPipelineProbeResult, error) {
	alerts, err := retrieveAlerts(ctx, p.getter)
	if err != nil {
		return OTelPipelineProbeResult{}, fmt.Errorf("failed to retrieve alerts: %w", err)
	}

	allDropped := p.isFiring(alerts, config.RuleNameAllDataDropped, pipelineName)
	someDropped := p.isFiring(alerts, config.RuleNameSomeDataDropped, pipelineName)
	queueAlmostFull := p.isFiring(alerts, config.RuleNameQueueAlmostFull, pipelineName)
	throttling := p.isFiring(alerts, config.RuleNameThrottling, pipelineName)
	healthy := !(allDropped || someDropped || queueAlmostFull || throttling)

	return OTelPipelineProbeResult{
		PipelineProbeResult: PipelineProbeResult{
			AllDataDropped:  allDropped,
			SomeDataDropped: someDropped,
			Healthy:         healthy,
		},
		QueueAlmostFull: queueAlmostFull,
		Throttling:      throttling,
	}, nil
}

func (p *OTelPipelineProber) isFiring(alerts []promv1.Alert, ruleName, pipelineName string) bool {
	return isFiringWithMatcher(alerts, ruleName, pipelineName, p.matcher)
}
