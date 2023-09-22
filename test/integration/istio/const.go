//go:build istio

package istio

import (
	"time"
)

const (
	timeout                   = time.Second * 60
	interval                  = time.Millisecond * 250
	telemetryDeliveryInterval = time.Second * 10

	defaultNamespaceName    = "default"
	kymaSystemNamespaceName = "kyma-system"
)
