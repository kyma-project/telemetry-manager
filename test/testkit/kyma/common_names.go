package kyma

import (
	"k8s.io/apimachinery/pkg/types"

	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
)

const (
	DefaultNamespaceName     = "default"
	SystemNamespaceName      = "kyma-system"
	IstioSystemNamespaceName = "istio-system"

	TelemetryManagerMetricsPort = 8080
)

var (
	TelemetryManagerName               = types.NamespacedName{Name: names.ManagerName, Namespace: SystemNamespaceName}
	TelemetryManagerMetricsServiceName = types.NamespacedName{Name: names.ManagerMetricsService, Namespace: SystemNamespaceName}
	TelemetryManagerWebhookServiceName = types.NamespacedName{Name: names.ManagerWebhookService, Namespace: SystemNamespaceName}

	MetricAgentName               = types.NamespacedName{Name: names.MetricAgent, Namespace: SystemNamespaceName}
	MetricAgentMetricsService     = types.NamespacedName{Name: names.MetricAgentMetricsService, Namespace: SystemNamespaceName}
	MetricAgentNetworkPolicy      = types.NamespacedName{Name: commonresources.NetworkPolicyPrefix + names.MetricAgent, Namespace: SystemNamespaceName}
	MetricAgentSecretName         = types.NamespacedName{Name: names.MetricAgent, Namespace: SystemNamespaceName}
	MetricAgentServiceAccount     = types.NamespacedName{Name: names.MetricAgent, Namespace: SystemNamespaceName}
	MetricAgentClusterRole        = types.NamespacedName{Name: names.MetricAgent, Namespace: SystemNamespaceName}
	MetricAgentClusterRoleBinding = types.NamespacedName{Name: names.MetricAgent, Namespace: SystemNamespaceName}
	MetricAgentConfigMap          = types.NamespacedName{Name: names.MetricAgent, Namespace: SystemNamespaceName}

	LogAgentName               = types.NamespacedName{Name: names.LogAgent, Namespace: SystemNamespaceName}
	LogAgentMetricsService     = types.NamespacedName{Name: names.LogAgentMetricsService, Namespace: SystemNamespaceName}
	LogAgentSecretName         = types.NamespacedName{Name: names.LogAgent, Namespace: SystemNamespaceName}
	LogAgentServiceAccount     = types.NamespacedName{Name: names.LogAgent, Namespace: SystemNamespaceName}
	LogAgentClusterRole        = types.NamespacedName{Name: names.LogAgent, Namespace: SystemNamespaceName}
	LogAgentClusterRoleBinding = types.NamespacedName{Name: names.LogAgent, Namespace: SystemNamespaceName}
	LogAgentNetworkPolicy      = types.NamespacedName{Name: commonresources.NetworkPolicyPrefix + names.LogAgent, Namespace: SystemNamespaceName}
	LogAgentConfigMap          = types.NamespacedName{Name: names.LogAgent, Namespace: SystemNamespaceName}

	FluentBitDaemonSetName          = types.NamespacedName{Name: names.FluentBit, Namespace: SystemNamespaceName}
	FluentBitServiceAccount         = types.NamespacedName{Name: names.FluentBit, Namespace: SystemNamespaceName}
	FluentBitClusterRole            = types.NamespacedName{Name: names.FluentBit, Namespace: SystemNamespaceName}
	FluentBitClusterRoleBinding     = types.NamespacedName{Name: names.FluentBit, Namespace: SystemNamespaceName}
	FluentBitExporterMetricsService = types.NamespacedName{Name: names.FluentBitExporterMetricsService, Namespace: SystemNamespaceName}
	FluentBitMetricsService         = types.NamespacedName{Name: names.FluentBitMetricsService, Namespace: SystemNamespaceName}
	FluentBitConfigMap              = types.NamespacedName{Name: names.FluentBit, Namespace: SystemNamespaceName}
	FluentBitSectionsConfigMap      = types.NamespacedName{Name: names.FluentBitSectionsConfigMap, Namespace: SystemNamespaceName}
	FluentBitLuaConfigMap           = types.NamespacedName{Name: names.FluentBitLuaScriptsConfigMap, Namespace: SystemNamespaceName}
	FluentBitFilesConfigMap         = types.NamespacedName{Name: names.FluentBitFilesConfigMap, Namespace: SystemNamespaceName}
	FluentBitNetworkPolicy          = types.NamespacedName{Name: commonresources.NetworkPolicyPrefix + names.FluentBit, Namespace: SystemNamespaceName}
	FluentBitEnvSecret              = types.NamespacedName{Name: names.FluentBit + "-env", Namespace: SystemNamespaceName}
	FluentBitTLSConfigSecret        = types.NamespacedName{Name: names.FluentBit + "-output-tls-config", Namespace: SystemNamespaceName}

	LogGatewayName                  = types.NamespacedName{Name: names.LogGateway, Namespace: SystemNamespaceName}    // TODO: Still needed for upgrade tests. Remove after first roll-out
	MetricGatewayName               = types.NamespacedName{Name: names.MetricGateway, Namespace: SystemNamespaceName} // TODO: Still needed for upgrade tests. Remove after first roll-out
	TraceGatewayName                = types.NamespacedName{Name: names.TraceGateway, Namespace: SystemNamespaceName}  // TODO: Still needed for upgrade tests. Remove after first roll-out
	OTLPGatewayName                 = types.NamespacedName{Name: names.OTLPGateway, Namespace: SystemNamespaceName}
	TelemetryOTLPMetricsService     = types.NamespacedName{Name: names.OTLPGatewayMetricsService, Namespace: SystemNamespaceName}
	TelemetryOTLPNetworkPolicy      = types.NamespacedName{Name: commonresources.NetworkPolicyPrefix + names.OTLPGateway, Namespace: SystemNamespaceName}
	TelemetryOTLPSecretName         = types.NamespacedName{Name: names.OTLPGateway, Namespace: SystemNamespaceName}
	TelemetryOTLPLogService         = types.NamespacedName{Name: names.OTLPLogsService, Namespace: SystemNamespaceName}
	TelemetryOTLPTraceService       = types.NamespacedName{Name: names.OTLPTracesService, Namespace: SystemNamespaceName}
	TelemetryOTLPMetricService      = types.NamespacedName{Name: names.OTLPMetricsService, Namespace: SystemNamespaceName}
	TelemetryOTLPServiceAccount     = types.NamespacedName{Name: names.OTLPGateway, Namespace: SystemNamespaceName}
	TelemetryOTLPClusterRole        = types.NamespacedName{Name: names.OTLPGateway, Namespace: SystemNamespaceName}
	TelemetryOTLPClusterRoleBinding = types.NamespacedName{Name: names.OTLPGateway, Namespace: SystemNamespaceName}
	TelemetryOTLPConfigMap          = types.NamespacedName{Name: names.OTLPGateway, Namespace: SystemNamespaceName}
	TelemetryOTLPRole               = types.NamespacedName{Name: names.OTLPGateway, Namespace: SystemNamespaceName}
	TelemetryOTLPRoleBinding        = types.NamespacedName{Name: names.OTLPGateway, Namespace: SystemNamespaceName}
	TelemetryOTLPPeerAuthentication = types.NamespacedName{Name: names.OTLPGateway, Namespace: SystemNamespaceName}

	SelfMonitorName           = types.NamespacedName{Name: names.SelfMonitor, Namespace: SystemNamespaceName}
	SelfMonitorNetworkPolicy  = types.NamespacedName{Name: commonresources.NetworkPolicyPrefix + names.SelfMonitor, Namespace: SystemNamespaceName}
	SelfMonitorServiceAccount = types.NamespacedName{Name: names.SelfMonitor, Namespace: SystemNamespaceName}
	SelfMonitorRole           = types.NamespacedName{Name: names.SelfMonitor, Namespace: SystemNamespaceName}
	SelfMonitorRoleBinding    = types.NamespacedName{Name: names.SelfMonitor, Namespace: SystemNamespaceName}
	SelfMonitorConfigMap      = types.NamespacedName{Name: names.SelfMonitor, Namespace: SystemNamespaceName}
	SelfMonitorService        = types.NamespacedName{Name: names.SelfMonitor, Namespace: SystemNamespaceName}

	TelemetryName = types.NamespacedName{Name: names.DefaultTelemetry, Namespace: SystemNamespaceName}

	WebhookCertSecret = types.NamespacedName{Name: "telemetry-webhook-cert", Namespace: SystemNamespaceName}
)
