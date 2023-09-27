package periodic

import (
	"time"
)

const (
	// DefaultTimeout is the default timeout duration used when no specialized interval is applicable.
	DefaultTimeout = time.Second * 60

	// NegativeCheckTimeout is used when asserting the absence of an event (typically with Consistently).
	NegativeCheckTimeout = time.Second * 10

	// TelemetryPollTimeout is used for asynchronous checks when polling Telemetry data from a mock backend via the export URL.
	// For example, to verify that a certain signal has provided resource attributes.
	TelemetryPollTimeout = time.Second * 40

	// DefaultInterval is the default interval duration used when no specialized interval is applicable.
	DefaultInterval = time.Millisecond * 250

	// TelemetryPollInterval is used for asynchronous checks when polling Telemetry data from a mock backend via the export URL.
	// For example, to verify that a certain signal has provided resource attributes.
	TelemetryPollInterval = time.Second
)
