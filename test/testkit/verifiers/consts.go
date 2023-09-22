package verifiers

import (
	"time"

	"k8s.io/apimachinery/pkg/types"
)

const (
	timeout               = time.Second * 60
	interval              = time.Millisecond * 250
	reconciliationTimeout = time.Second * 10
)

var (
	metricGatewayName = types.NamespacedName{Name: "telemetry-metric-gateway", Namespace: "kyma-system"}
	metricAgentName   = types.NamespacedName{Name: "telemetry-metric-agent", Namespace: "kyma-system"}
	traceGatewayName  = types.NamespacedName{Name: "telemetry-trace-collector", Namespace: "kyma-system"}
)
