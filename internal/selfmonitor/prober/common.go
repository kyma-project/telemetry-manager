package prober

import (
	"context"
	"time"
	"fmt"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"k8s.io/apimachinery/pkg/types"
	"github.com/prometheus/client_golang/api"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/ports"
)

const (
	clientTimeout = 10 * time.Second
)

//go:generate mockery --name alertGetter --filename=alert_getter.go --exported
type alertGetter interface {
	Alerts(ctx context.Context) (promv1.AlertsResult, error)
}

type PipelineProbeResult struct {
	AllDataDropped  bool
	SomeDataDropped bool
	Healthy         bool
}

func newPrometheusClient(selfMonitorName types.NamespacedName) (promv1.API, error) {
	client, err := api.NewClient(api.Config{
		Address: fmt.Sprintf("http://%s.%s:%d", selfMonitorName.Name, selfMonitorName.Namespace, ports.PrometheusPort),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus client: %w", err)
	}
	return promv1.NewAPI(client), nil
}

func retrieveAlerts(ctx context.Context, getter alertGetter) ([]promv1.Alert, error) {
	childCtx, cancel := context.WithTimeout(ctx, clientTimeout)
	defer cancel()

	result, err := getter.Alerts(childCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to query Prometheus alerts: %w", err)
	}

	return result.Alerts, nil
}
