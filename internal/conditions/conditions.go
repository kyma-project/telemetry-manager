package conditions

const (
	TypeAgentHealthy            = "AgentHealthy"
	TypeConfigurationGenerated  = "ConfigurationGenerated"
	TypeFlowHealthy             = "TelemetryFlowHealthy"
	TypeGatewayHealthy          = "GatewayHealthy"
	TypeLogComponentsHealthy    = "LogComponentsHealthy"
	TypeMetricComponentsHealthy = "MetricComponentsHealthy"
	TypeTraceComponentsHealthy  = "TraceComponentsHealthy"

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
	// Telemetry reasons
	ReasonComponentsRunning      = "ComponentsRunning"
	ReasonNoPipelineDeployed     = "NoPipelineDeployed"
	ReasonResourceBlocksDeletion = "ResourceBlocksDeletion"

	// Common reasons
	ReasonAgentNotReady = "AgentNotReady"
	ReasonAgentReady    = "AgentReady"
	ReasonGatewayNotReady             = "GatewayNotReady"
	ReasonGatewayReady                = "GatewayReady"
	ReasonMaxPipelinesExceeded        = "MaxPipelinesExceeded"
	ReasonReferencedSecretMissing     = "ReferencedSecretMissing"
	ReasonSelfMonAllDataDropped       = "AllTelemetryDataDropped"
	ReasonSelfMonBufferFillingUp      = "BufferFillingUp"
	ReasonSelfMonFlowHealthy          = "FlowHealthy"
	ReasonSelfMonGatewayThrottling    = "GatewayThrottling"
	ReasonSelfMonSomeDataDropped      = "SomeTelemetryDataDropped"
	ReasonTLSCertificateAboutToExpire = "TLSCertificateAboutToExpire"
	ReasonTLSCertificateExpired       = "TLSCertificateExpired"
	ReasonTLSCertificateInvalid       = "TLSCertificateInvalid"

	// LogPipeline reasons
	ReasonAgentConfigured        = "AgentConfigured"
	ReasonSelfMonNoLogsDelivered = "NoLogsDelivered"
	ReasonUnsupportedLokiOutput  = "UnsupportedLokiOutput"

	// TracePipeline reasons
	ReasonGatewayConfigured = "GatewayConfigured"

	// MetricPipeline reasons
	ReasonAgentGatewayConfigured = "AgentGatewayConfigured"
	ReasonMetricAgentNotRequired = "AgentNotRequired"

	// NOTE: The "FluentBitDaemonSetNotReady", "FluentBitDaemonSetReady", "TraceGatewayDeploymentNotReady" and "TraceGatewayDeploymentReady" reasons are deprecated.
	// They will be removed when the "Running" and "Pending" types are removed
	// Check https://github.com/kyma-project/telemetry-manager/blob/main/docs/contributor/arch/004-consolidate-pipeline-statuses.md#decision
	ReasonFluentBitDSNotReady            = "FluentBitDaemonSetNotReady"
	ReasonFluentBitDSReady               = "FluentBitDaemonSetReady"
	ReasonTraceGatewayDeploymentNotReady = "TraceGatewayDeploymentNotReady"
	ReasonTraceGatewayDeploymentReady    = "TraceGatewayDeploymentReady"
)

var commonMessages = map[string]string{
	ReasonMaxPipelinesExceeded:        "Maximum pipeline count limit exceeded",
	ReasonNoPipelineDeployed:          "No pipelines have been deployed",
	ReasonReferencedSecretMissing:     "One or more referenced Secrets are missing",
	ReasonTLSCertificateAboutToExpire: "TLS certificate is about to expire, configured certificate is valid until %s",
	ReasonTLSCertificateExpired:       "TLS certificate expired on %s",
	ReasonTLSCertificateInvalid:       "TLS certificate invalid: %s",
}

var logPipelineMessages = map[string]string{
	ReasonAgentConfigured:        "Fluent Bit agent successfully configured",
	ReasonAgentNotReady:          "Fluent Bit agent DaemonSet is not ready",
	ReasonAgentReady:             "Fluent Bit agent DaemonSet is ready",
	ReasonComponentsRunning:      "All log components are running",
	ReasonFluentBitDSNotReady:    "Fluent Bit DaemonSet is not ready",
	ReasonFluentBitDSReady:       "Fluent Bit DaemonSet is ready",
	ReasonSelfMonAllDataDropped:  "All logs dropped: backend unreachable or rejecting",
	ReasonSelfMonBufferFillingUp: "Buffer nearing capacity: incoming log rate exceeds export rate",
	ReasonSelfMonFlowHealthy:     "No problems detected in the log flow",
	ReasonSelfMonNoLogsDelivered: "No logs delivered to backend",
	ReasonSelfMonSomeDataDropped: "Some logs dropped: backend unreachable or rejecting",
	ReasonUnsupportedLokiOutput:  "grafana-loki output is not supported anymore. For integration with a custom Loki installation, use the `custom` output and follow https://kyma-project.io/#/telemetry-manager/user/integration/loki/README",
}

var tracePipelineMessages = map[string]string{
	ReasonGatewayConfigured:              "Trace gateway successfully configured",
	ReasonComponentsRunning:              "All trace components are running",
	ReasonGatewayNotReady:                "Trace gateway Deployment is not ready",
	ReasonGatewayReady:                   "Trace gateway Deployment is ready",
	ReasonSelfMonAllDataDropped:          "All traces dropped: backend unreachable or rejecting. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/03-traces?id=traces-not-arriving-at-the-destination",
	ReasonSelfMonBufferFillingUp:         "Buffer nearing capacity: incoming trace rate exceeds export rate",
	ReasonSelfMonFlowHealthy:             "No problems detected in the trace flow",
	ReasonSelfMonGatewayThrottling:       "Trace gateway experiencing high influx: unable to receive traces at current rate",
	ReasonSelfMonSomeDataDropped:         "Some traces dropped: backend unreachable or rejecting. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/03-traces?id=traces-not-arriving-at-the-destination",
	ReasonTraceGatewayDeploymentNotReady: "Trace gateway Deployment is not ready",
	ReasonTraceGatewayDeploymentReady:    "Trace gateway Deployment is ready",
}

var metricPipelineMessages = map[string]string{
	ReasonAgentGatewayConfigured:   "Metric agent and gateway successfully configured",
	ReasonAgentNotReady:            "Metric agent DaemonSet is not ready",
	ReasonAgentReady:               "Metric agent DaemonSet is ready",
	ReasonComponentsRunning:        "All metric components are running",
	ReasonGatewayNotReady:          "Metric gateway Deployment is not ready",
	ReasonGatewayReady:             "Metric gateway Deployment is ready",
	ReasonSelfMonAllDataDropped:    "All metrics dropped: backend unreachable or rejecting. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/04-metrics?id=metrics-not-arriving-at-the-destination",
	ReasonSelfMonBufferFillingUp:   "Buffer nearing capacity: incoming metric rate exceeds export rate",
	ReasonSelfMonFlowHealthy:       "No problems detected in the metric flow",
	ReasonSelfMonGatewayThrottling: "Metric gateway experiencing high influx: unable to receive metrics at current rate",
	ReasonSelfMonSomeDataDropped:   "Some metrics dropped: backend unreachable or rejecting. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/04-metrics?id=metrics-not-arriving-at-the-destination",
}

func MessageForLogPipeline(reason string) string {
	return message(reason, logPipelineMessages)
}

func MessageForTracePipeline(reason string) string {
	return message(reason, tracePipelineMessages)
}

func MessageForMetricPipeline(reason string) string {
	return message(reason, metricPipelineMessages)
}

func message(reason string, specializedMessages map[string]string) string {
	if condMessage, found := commonMessages[reason]; found {
		return condMessage
	}
	if condMessage, found := specializedMessages[reason]; found {
		return condMessage
	}
	return ""
}
