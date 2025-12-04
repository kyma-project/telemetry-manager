package prober

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/internal/metrics"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/ports"
)

const (
	clientTimeout = 10 * time.Second
)

func newPrometheusClient(selfMonitorName types.NamespacedName) (promv1.API, error) {
	client, err := api.NewClient(api.Config{
		Address: fmt.Sprintf("http://%s.%s:%d", selfMonitorName.Name, selfMonitorName.Namespace, ports.PrometheusPort),
		Client:  newInstrumentedClient(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus client: %w", err)
	}

	return promv1.NewAPI(client), nil
}

func newInstrumentedClient() *http.Client {
	return &http.Client{
		Timeout: clientTimeout,
		Transport: promhttp.InstrumentRoundTripperInFlight(metrics.SelfMonitorProberRequestsInFlight,
			promhttp.InstrumentRoundTripperCounter(metrics.SelfMonitorProberRequestsTotal,
				promhttp.InstrumentRoundTripperDuration(metrics.SelfMonitorProberRequestDuration, http.DefaultTransport),
			),
		),
	}
}
