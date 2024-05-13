package prober

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/ports"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	metricPrefix  = "telemetry_self_monitor_prober_"
	clientTimeout = 10 * time.Second
)

var (
	requestsInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: metricPrefix + "in_flight_requests",
		Help: "The current number of in-flight requests initiated by the self-monitoring prober.",
	})

	requestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: metricPrefix + "requests_total",
			Help: "Total number of requests initiated by the self-monitoring prober.",
		},
		[]string{"code", "method"},
	)

	requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "telemetry_self_monitor_prober_duration_seconds",
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
