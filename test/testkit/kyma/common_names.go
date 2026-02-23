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

	MetricGatewayName               = types.NamespacedName{Name: names.MetricGateway, Namespace: SystemNamespaceName}
	MetricGatewayMetricsService     = types.NamespacedName{Name: names.MetricGatewayMetricsService, Namespace: SystemNamespaceName}
	MetricGatewayNetworkPolicy      = types.NamespacedName{Name: commonresources.NetworkPolicyPrefix + names.MetricGateway, Namespace: SystemNamespaceName}
	MetricGatewaySecretName         = types.NamespacedName{Name: names.MetricGateway, Namespace: SystemNamespaceName}
	MetricGatewayOTLPService        = types.NamespacedName{Name: names.OTLPMetricsService, Namespace: SystemNamespaceName}
	MetricGatewayServiceAccount     = types.NamespacedName{Name: names.MetricGateway, Namespace: SystemNamespaceName}
	MetricGatewayClusterRole        = types.NamespacedName{Name: names.MetricGateway, Namespace: SystemNamespaceName}
	MetricGatewayClusterRoleBinding = types.NamespacedName{Name: names.MetricGateway, Namespace: SystemNamespaceName}
	MetricGatewayConfigMap          = types.NamespacedName{Name: names.MetricGateway, Namespace: SystemNamespaceName}
	MetricGatewayRole               = types.NamespacedName{Name: names.MetricGateway, Namespace: SystemNamespaceName}
	MetricGatewayRoleBinding        = types.NamespacedName{Name: names.MetricGateway, Namespace: SystemNamespaceName}

	MetricAgentName               = types.NamespacedName{Name: names.MetricAgent, Namespace: SystemNamespaceName}
	MetricAgentMetricsService     = types.NamespacedName{Name: names.MetricAgentMetricsService, Namespace: SystemNamespaceName}
	MetricAgentNetworkPolicy      = types.NamespacedName{Name: commonresources.NetworkPolicyPrefix + names.MetricAgent, Namespace: SystemNamespaceName}
	MetricAgentSecretName         = types.NamespacedName{Name: names.MetricAgent, Namespace: SystemNamespaceName}
	MetricAgentServiceAccount     = types.NamespacedName{Name: names.MetricAgent, Namespace: SystemNamespaceName}
	MetricAgentClusterRole        = types.NamespacedName{Name: names.MetricAgent, Namespace: SystemNamespaceName}
	MetricAgentClusterRoleBinding = types.NamespacedName{Name: names.MetricAgent, Namespace: SystemNamespaceName}
	MetricAgentConfigMap          = types.NamespacedName{Name: names.MetricAgent, Namespace: SystemNamespaceName}

	TraceGatewayName               = types.NamespacedName{Name: names.TraceGateway, Namespace: SystemNamespaceName}
	TraceGatewayMetricsService     = types.NamespacedName{Name: names.TraceGatewayMetricsService, Namespace: SystemNamespaceName}
	TraceGatewayNetworkPolicy      = types.NamespacedName{Name: commonresources.NetworkPolicyPrefix + names.TraceGateway, Namespace: SystemNamespaceName}
	TraceGatewaySecretName         = types.NamespacedName{Name: names.TraceGateway, Namespace: SystemNamespaceName}
	TraceGatewayOTLPService        = types.NamespacedName{Name: names.OTLPTracesService, Namespace: SystemNamespaceName}
	TraceGatewayServiceAccount     = types.NamespacedName{Name: names.TraceGateway, Namespace: SystemNamespaceName}
	TraceGatewayClusterRole        = types.NamespacedName{Name: names.TraceGateway, Namespace: SystemNamespaceName}
	TraceGatewayClusterRoleBinding = types.NamespacedName{Name: names.TraceGateway, Namespace: SystemNamespaceName}
	TraceGatewayConfigMap          = types.NamespacedName{Name: names.TraceGateway, Namespace: SystemNamespaceName}

	LogAgentName               = types.NamespacedName{Name: names.LogAgent, Namespace: SystemNamespaceName}
	LogAgentMetricsService     = types.NamespacedName{Name: names.LogAgentMetricsService, Namespace: SystemNamespaceName}
	LogAgentServiceAccount     = types.NamespacedName{Name: names.LogAgent, Namespace: SystemNamespaceName}
	LogAgentClusterRole        = types.NamespacedName{Name: names.LogAgent, Namespace: SystemNamespaceName}
	LogAgentClusterRoleBinding = types.NamespacedName{Name: names.LogAgent, Namespace: SystemNamespaceName}
	LogAgentNetworkPolicy      = types.NamespacedName{Name: commonresources.NetworkPolicyPrefix + names.LogAgent, Namespace: SystemNamespaceName}
	LogAgentConfigMap          = types.NamespacedName{Name: names.LogAgent, Namespace: SystemNamespaceName}

	LogGatewayName               = types.NamespacedName{Name: names.LogGateway, Namespace: SystemNamespaceName}
	LogGatewayMetricsService     = types.NamespacedName{Name: names.LogGatewayMetricsService, Namespace: SystemNamespaceName}
	LogGatewayNetworkPolicy      = types.NamespacedName{Name: commonresources.NetworkPolicyPrefix + names.LogGateway, Namespace: SystemNamespaceName}
	LogGatewaySecretName         = types.NamespacedName{Name: names.LogGateway, Namespace: SystemNamespaceName}
	LogGatewayOTLPService        = types.NamespacedName{Name: names.OTLPLogsService, Namespace: SystemNamespaceName}
	LogGatewayServiceAccount     = types.NamespacedName{Name: names.LogGateway, Namespace: SystemNamespaceName}
	LogGatewayClusterRole        = types.NamespacedName{Name: names.LogGateway, Namespace: SystemNamespaceName}
	LogGatewayClusterRoleBinding = types.NamespacedName{Name: names.LogGateway, Namespace: SystemNamespaceName}
	LogGatewayConfigMap          = types.NamespacedName{Name: names.LogGateway, Namespace: SystemNamespaceName}

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

	SelfMonitorName          = types.NamespacedName{Name: names.SelfMonitor, Namespace: SystemNamespaceName}
	SelfMonitorNetworkPolicy = types.NamespacedName{Name: commonresources.NetworkPolicyPrefix + names.SelfMonitor, Namespace: SystemNamespaceName}

	TelemetryName = types.NamespacedName{Name: names.DefaultTelemetry, Namespace: SystemNamespaceName}

	WebhookCertSecret = types.NamespacedName{Name: "telemetry-webhook-cert", Namespace: SystemNamespaceName}

	TelemetryOTLPGatewayName        = types.NamespacedName{Name: names.OTLPGateway, Namespace: SystemNamespaceName}
	TelemetryOTLPMetricsService     = types.NamespacedName{Name: names.OTLPGateway + "-metrics", Namespace: SystemNamespaceName}
	TelemetryOTLPServiceAccount     = types.NamespacedName{Name: names.OTLPGateway, Namespace: SystemNamespaceName}
	TelemetryOTLPClusterRole        = types.NamespacedName{Name: names.OTLPGateway, Namespace: SystemNamespaceName}
	TelemetryOTLPClusterRoleBinding = types.NamespacedName{Name: names.OTLPGateway, Namespace: SystemNamespaceName}
	TelemetryOTLPNetworkPolicy      = types.NamespacedName{Name: commonresources.NetworkPolicyPrefix + names.OTLPGateway, Namespace: SystemNamespaceName}
	TelemetryOTLPSecretName         = types.NamespacedName{Name: names.OTLPGateway, Namespace: SystemNamespaceName}
	TelemetryOTLPConfigMap          = types.NamespacedName{Name: names.OTLPGateway, Namespace: SystemNamespaceName}
	TelemetryOTLPOTLPService        = types.NamespacedName{Name: names.OTLPGateway, Namespace: SystemNamespaceName}
)
