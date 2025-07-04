package main

import (
	"net/http"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const port = 4317

func main() {
	receiver := NewOTLPDataReceiver(port)
	mockbackend := testbed.NewMockBackend("mockbackend.log", receiver)
	mockbackend.Start()

	// Register a Prometheus gauge that reflects DataItemsReceived
	dataItemsReceivedGauge := prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "mockbackend_data_items_received",
			Help: "Number of data items received by the mock backend",
		},
		func() float64 {
			return float64(mockbackend.DataItemsReceived())
		},
	)
	prometheus.MustRegister(dataItemsReceivedGauge)

	// Expose /metrics endpoint
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":2112", nil)
	}()

	// Block forever so the container doesn't exit
	select {}
}
