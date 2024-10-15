package prober

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/ports"
)

const (
	metricPrefix  = "telemetry_self_monitor_prober_"
	clientTimeout = 10 * time.Second
)

var (
	requestsInFlight = promauto.With(metrics.Registry).NewGauge(
		prometheus.GaugeOpts{
			Name: metricPrefix + "in_flight_requests",
			Help: "The current number of in-flight requests initiated by the self-monitoring prober.",
		},
	)

	requestsTotal = promauto.With(metrics.Registry).NewCounterVec(
		prometheus.CounterOpts{
			Name: metricPrefix + "requests_total",
			Help: "Total number of requests initiated by the self-monitoring prober.",
		},
		[]string{"code"},
	)

	requestDuration = promauto.With(metrics.Registry).NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    metricPrefix + "duration_seconds",
			Help:    "A histogram of latencies for requests initiated by the self-monitoring prober.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{},
	)
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
		Transport: promhttp.InstrumentRoundTripperInFlight(requestsInFlight,
			promhttp.InstrumentRoundTripperCounter(requestsTotal,
				promhttp.InstrumentRoundTripperDuration(requestDuration, http.DefaultTransport),
			),
		),
	}
}
