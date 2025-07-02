package main

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

const (
	dataItemsPerSecond = 10_000
	itemsPerBatch      = 10
	host               = "telemetry-otlp-logs.kyma-system"
	port               = 4317
)

func main() {
	options := testbed.LoadOptions{DataItemsPerSecond: dataItemsPerSecond, ItemsPerBatch: itemsPerBatch}

	dataProvider := testbed.NewPerfTestDataProvider(options)
	// dataSender := testbed.NewOTLPLogsDataSender(host, port)
	dataSender := NewFileLogWriter()
	loadGenerator, err := testbed.NewLoadGenerator(dataProvider, dataSender)
	if err != nil {
		panic(fmt.Errorf("failed to create load generator: %w", err))
	}

	loadGenerator.Start(options)

	// Block forever so the container doesn't exit
	select {}
}
