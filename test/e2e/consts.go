//go:build e2e

package e2e

import (
	"time"
)

const (
	timeout  = time.Second * 60
	interval = time.Millisecond * 250

	kymaSystemNamespace = "kyma-system"
	mocksNamespace      = "mocks"
	telemetryNamespace  = "default"
)
