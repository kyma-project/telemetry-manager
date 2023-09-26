package periodic

import (
	"time"
)

const (
	Timeout              = time.Second * 60
	NegativeCheckTimeout = time.Second * 10
	TelemetryPollTimeout = time.Second * 40

	Interval              = time.Millisecond * 250
	TelemetryPollInterval = time.Second
)
