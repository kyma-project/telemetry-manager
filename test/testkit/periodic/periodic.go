package periodic

import (
	"time"
)

const (
	// EventuallyTimeout is used when asserting an event with Eventually. Should be larger than periodic reconciliation interval.
	EventuallyTimeout = time.Second * 120

	// ConsistentlyTimeout is used when asserting the absence of an event with Consistently.
	ConsistentlyTimeout = time.Second * 10

	// TelemetryEventuallyTimeout is used for asynchronous checks when polling Telemetry data from a mock backend via the export URL.
	// For example, to verify that a certain signal has provided resource attributes.
	TelemetryEventuallyTimeout = time.Second * 90

	// TelemetryConsistentlyTimeout is used for asynchronous checks when polling Telemetry data from a mock backend via the export URL.
	// For example, to verify that a certain signal *does not* have provided resource attributes.
	TelemetryConsistentlyTimeout = time.Second * 20

	// DefaultInterval is the default interval duration used when no specialized interval is applicable.
	DefaultInterval = time.Millisecond * 250

	// TelemetryInterval is used for asynchronous checks when polling Telemetry data from a mock backend via the export URL.
	TelemetryInterval = time.Second
)
