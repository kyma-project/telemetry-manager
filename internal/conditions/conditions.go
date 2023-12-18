package conditions

const (
	TypeMetricGatewayHealthy   = "GatewayHealthy"
	TypeMetricAgentHealthy     = "AgentHealthy"
	TypeConfigurationGenerated = "ConfigurationGenerated"
)

const (
	ReasonNoPipelineDeployed      = "NoPipelineDeployed"
	ReasonReferencedSecretMissing = "ReferencedSecretMissing"
	ReasonWaitingForLock          = "WaitingForLock"
	ReasonResourceBlocksDeletion  = "ResourceBlocksDeletion"

	ReasonFluentBitDSNotReady   = "FluentBitDaemonSetNotReady"
	ReasonFluentBitDSReady      = "FluentBitDaemonSetReady"
	ReasonUnsupportedLokiOutput = "UnsupportedLokiOutput"

	ReasonMetricGatewayDeploymentNotReady = "DeploymentNotReady"
	ReasonMetricGatewayDeploymentReady    = "DeploymentReady"
	ReasonMetricAgentDaemonSetNotReady    = "DaemonSetNotReady"
	ReasonMetricAgentDaemonSetReady       = "DaemonSetReady"
	ReasonMetricAgentNotRequired          = "AgentNotRequired"
	ReasonMetricConfigurationGenerated    = "ConfigurationGenerated"
	ReasonMetricComponentsRunning         = "MetricComponentsRunning"

	ReasonTraceGatewayDeploymentNotReady = "TraceGatewayDeploymentNotReady"
	ReasonTraceGatewayDeploymentReady    = "TraceGatewayDeploymentReady"
)

var message = map[string]string{
	ReasonNoPipelineDeployed:      "No pipelines have been deployed",
	ReasonReferencedSecretMissing: "One or more referenced Secrets are missing",
	ReasonWaitingForLock:          "Waiting for the lock",

	ReasonFluentBitDSNotReady:   "Fluent Bit DaemonSet is not ready",
	ReasonFluentBitDSReady:      "Fluent Bit DaemonSet is ready",
	ReasonUnsupportedLokiOutput: "grafana-loki output is not supported anymore. For integration with a custom Loki installation, use the `custom` output and follow https://github.com/kyma-project/examples/tree/main/loki",

	ReasonMetricGatewayDeploymentNotReady: "Metric gateway Deployment is not ready",
	ReasonMetricGatewayDeploymentReady:    "Metric gateway Deployment is ready",
	ReasonMetricAgentDaemonSetNotReady:    "Metric agent DaemonSet is not ready",
	ReasonMetricAgentDaemonSetReady:       "Metric agent DaemonSet is ready",
	ReasonMetricComponentsRunning:         "All metric components are running",

	ReasonTraceGatewayDeploymentNotReady: "Trace gateway Deployment is not ready",
	ReasonTraceGatewayDeploymentReady:    "Trace gateway Deployment is ready",
}

// CommonMessageFor returns a human-readable message corresponding to a given reason.
// In more advanced scenarios, you may craft custom messages tailored to specific use cases.
func CommonMessageFor(reason string) string {
	if condMessage, found := message[reason]; found {
		return condMessage
	}
	return ""
}
