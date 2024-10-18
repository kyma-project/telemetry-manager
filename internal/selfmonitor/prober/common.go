package prober

import (
	"context"
	"fmt"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type alertGetter interface {
	Alerts(ctx context.Context) (promv1.AlertsResult, error)
}

type PipelineProbeResult struct {
	AllDataDropped  bool
	SomeDataDropped bool
	Healthy         bool
}

type matcherFunc func(alertLabels map[string]string, expectedRuleName string, expectedPipelineName string) bool

func retrieveAlerts(ctx context.Context, getter alertGetter) ([]promv1.Alert, error) {
	result, err := getter.Alerts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query Prometheus alerts: %w", err)
	}

	logf.FromContext(ctx).V(1).Info("Retrieved alerts", "alerts", result.Alerts)

	return result.Alerts, nil
}

func isFiringWithMatcher(alerts []promv1.Alert, ruleName, pipelineName string, mf matcherFunc) bool {
	for _, alert := range alerts {
		if alert.State == promv1.AlertStateFiring && mf(toRawLabels(alert.Labels), ruleName, pipelineName) {
			return true
		}
	}

	return false
}

func toRawLabels(ls model.LabelSet) map[string]string {
	rawLabels := make(map[string]string, len(ls))
	for k, v := range ls {
		rawLabels[string(k)] = string(v)
	}

	return rawLabels
}
