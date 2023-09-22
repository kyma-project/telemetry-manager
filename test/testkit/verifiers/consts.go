package verifiers

import (
	"time"

	"k8s.io/apimachinery/pkg/types"
)

const (
	timeout               = time.Second * 60
	interval              = time.Millisecond * 250
	reconciliationTimeout = time.Second * 10

	defaultNamespaceName    = "default"
	kymaSystemNamespaceName = "kyma-system"

	metricGatewayBaseName = "telemetry-metric-gateway"
	metricAgentBaseName   = "telemetry-metric-agent"

	traceCollectorBaseName = "telemetry-trace-collector"
)

var (
	metricGatewayName  = types.NamespacedName{Name: metricGatewayBaseName, Namespace: kymaSystemNamespaceName}
	metricAgentName    = types.NamespacedName{Name: metricAgentBaseName, Namespace: kymaSystemNamespaceName}
	traceCollectorName = types.NamespacedName{Name: traceCollectorBaseName, Namespace: kymaSystemNamespaceName}
)
