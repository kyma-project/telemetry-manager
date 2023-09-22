//go:build e2e

package e2e

import (
	"time"
)

const (
	timeout                  = time.Second * 60
	reconciliationTimeout    = time.Second * 10
	telemetryDeliveryTimeout = time.Second * 20
	interval                 = time.Millisecond * 250
)
