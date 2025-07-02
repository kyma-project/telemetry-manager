package main

import (
	"fmt"

	"net/http"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	dataItemsPerSecond = 1000
	itemsPerBatch      = 10
	host               = "telemetry-otlp-logs.kyma-system"
	port               = 4317
)

func main() {
	options := testbed.LoadOptions{
		DataItemsPerSecond: dataItemsPerSecond,
		ItemsPerBatch:      itemsPerBatch,
	}

	dataProvider := testbed.NewPerfTestDataProvider(options)
	// dataSender := testbed.NewOTLPLogsDataSender(host, port)
	dataSender := NewFileLogWriter()
	loadGenerator, err := testbed.NewLoadGenerator(dataProvider, dataSender)
	if err != nil {
		panic(fmt.Errorf("failed to create load generator: %w", err))
	}

	loadGenerator.Start(options)

	// Register a Prometheus gauge that reflects DataItemsSent
	dataItemsSentGauge := prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "loadgenerator_data_items_sent",
			Help: "Number of data items sent by the load generator",
		},
		func() float64 {
			return float64(loadGenerator.DataItemsSent())
		},
	)
	prometheus.MustRegister(dataItemsSentGauge)

	// Expose /metrics endpoint
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":2112", nil)
	}()

	// Block forever so the container doesn't exit
	select {}
}
