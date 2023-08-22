//go:build istio

package istio

import (
	"time"
)

const (
	timeout  = time.Second * 60
	interval = time.Millisecond * 250

	// The filename for the OpenTelemetry collector's file exporter.
	telemetryDataFilename = "otlp-data.jsonl"

	defaultNamespaceName    = "default"
	kymaSystemNamespaceName = "kyma-system"
)
