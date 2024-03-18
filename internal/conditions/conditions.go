package conditions

const (
	TypeGatewayHealthy         = "GatewayHealthy"
	TypeAgentHealthy           = "AgentHealthy"
	TypeConfigurationGenerated = "ConfigurationGenerated"
	TypeFlowHealthy            = "TelemetryFlowHealthy"

	// NOTE: The "Running" and "Pending" types are deprecated
	// Check https://github.com/kyma-project/telemetry-manager/blob/main/docs/contributor/arch/004-consolidate-pipeline-statuses.md#decision
	TypeRunning = "Running"
	TypePending = "Pending"
)

const (
	RunningTypeDeprecationMsg = "[NOTE: The \"Running\" type is deprecated] "
	PendingTypeDeprecationMsg = "[NOTE: The \"Pending\" type is deprecated] "
)

const (
	ReasonNoPipelineDeployed      = "NoPipelineDeployed"
	ReasonReferencedSecretMissing = "ReferencedSecretMissing"
	ReasonMaxPipelinesExceeded    = "MaxPipelinesExceeded"
	ReasonResourceBlocksDeletion  = "ResourceBlocksDeletion"
	ReasonConfigurationGenerated  = "ConfigurationGenerated"
	ReasonDeploymentNotReady      = "DeploymentNotReady"
	ReasonDeploymentReady         = "DeploymentReady"
	ReasonDaemonSetNotReady       = "DaemonSetNotReady"
	ReasonDaemonSetReady          = "DaemonSetReady"
	ReasonAllDataDropped          = "AllTelemetryDataDropped"
	ReasonSomeDataDropped         = "SomeTelemetryDataDropped"
	ReasonBufferFillingUp         = "BufferFillingUp"
	ReasonGatewayThrottling       = "GatewayThrottling"
	ReasonFlowHealthy             = "Healthy"

	ReasonMetricAgentNotRequired  = "AgentNotRequired"
	ReasonMetricComponentsRunning = "MetricComponentsRunning"

	ReasonUnsupportedLokiOutput = "UnsupportedLokiOutput"
	ReasonLogComponentsRunning  = "LogComponentsRunning"

	ReasonTraceComponentsRunning = "TraceComponentsRunning"

	// NOTE: The "FluentBitDaemonSetNotReady", "FluentBitDaemonSetReady", "TraceGatewayDeploymentNotReady" and "TraceGatewayDeploymentReady" reasons are deprecated.
	// They will be removed when the "Running" and "Pending" types are removed
	// Check https://github.com/kyma-project/telemetry-manager/blob/main/docs/contributor/arch/004-consolidate-pipeline-statuses.md#decision
	ReasonFluentBitDSNotReady            = "FluentBitDaemonSetNotReady"
	ReasonFluentBitDSReady               = "FluentBitDaemonSetReady"
	ReasonTraceGatewayDeploymentNotReady = "TraceGatewayDeploymentNotReady"
	ReasonTraceGatewayDeploymentReady    = "TraceGatewayDeploymentReady"
)

var commonMessages = map[string]string{
	ReasonNoPipelineDeployed:      "No pipelines have been deployed",
	ReasonReferencedSecretMissing: "One or more referenced Secrets are missing",
	ReasonMaxPipelinesExceeded:    "Maximum pipeline count limit exceeded",
}

var metricPipelineMessages = map[string]string{
	ReasonDeploymentNotReady:      "Metric gateway Deployment is not ready",
	ReasonDeploymentReady:         "Metric gateway Deployment is ready",
	ReasonDaemonSetNotReady:       "Metric agent DaemonSet is not ready",
	ReasonDaemonSetReady:          "Metric agent DaemonSet is ready",
	ReasonMetricComponentsRunning: "All metric components are running",
}

var TracesMessage = map[string]string{
	ReasonDeploymentNotReady:             "Trace gateway Deployment is not ready",
	ReasonDeploymentReady:                "Trace gateway Deployment is ready",
	ReasonTraceGatewayDeploymentNotReady: "Trace gateway Deployment is not ready",
	ReasonTraceGatewayDeploymentReady:    "Trace gateway Deployment is ready",
	ReasonTraceComponentsRunning:         "All trace components are running",
}

var LogsMessage = map[string]string{
	ReasonDaemonSetNotReady:     "Fluent Bit DaemonSet is not ready",
	ReasonDaemonSetReady:        "Fluent Bit DaemonSet is ready",
	ReasonFluentBitDSNotReady:   "Fluent Bit DaemonSet is not ready",
	ReasonFluentBitDSReady:      "Fluent Bit DaemonSet is ready",
	ReasonUnsupportedLokiOutput: "grafana-loki output is not supported anymore. For integration with a custom Loki installation, use the `custom` output and follow https://kyma-project.io/#/telemetry-manager/user/integration/loki/README",
	ReasonLogComponentsRunning:  "All log components are running",
}

// MessageFor returns a human-readable message corresponding to a given reason.
// In more advanced scenarios, you may craft custom messages tailored to specific use cases.
func MessageFor(reason string, messageMap map[string]string) string {
	if condMessage, found := commonMessages[reason]; found {
		return condMessage
	}
	if condMessage, found := messageMap[reason]; found {
		return condMessage
	}
	return ""
}

func MessageForLogPipeline(reason string) string {
	return message(reason, commonMessages, LogsMessage)
}

func MessageForTracePipeline(reason string) string {
	return message(reason, commonMessages, TracesMessage)
}

func MessageForMetricPipeline(reason string) string {
	return message(reason, commonMessages, metricPipelineMessages)
}

func message(reason string, commonMessages, specializedMessages map[string]string) string {
	if condMessage, found := commonMessages[reason]; found {
		return condMessage
	}
	if condMessage, found := specializedMessages[reason]; found {
		return condMessage
	}
	return ""
}
