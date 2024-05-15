package conditions

const (
	TypeGatewayHealthy          = "GatewayHealthy"
	TypeAgentHealthy            = "AgentHealthy"
	TypeConfigurationGenerated  = "ConfigurationGenerated"
	TypeFlowHealthy             = "TelemetryFlowHealthy"
	TypeLogComponentsHealthy    = "LogComponentsHealthy"
	TypeTraceComponentsHealthy  = "TraceComponentsHealthy"
	TypeMetricComponentsHealthy = "MetricComponentsHealthy"

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
	// Universal pipeline reasons
	ReasonNoPipelineDeployed      = "NoPipelineDeployed"
	ReasonComponentsRunning       = "ComponentsRunning"
	ReasonReferencedSecretMissing = "ReferencedSecretMissing"
	ReasonResourceBlocksDeletion  = "ResourceBlocksDeletion"

	// Common pipeline reasons
	ReasonMaxPipelinesExceeded        = "MaxPipelinesExceeded"
	ReasonConfigurationGenerated      = "ConfigurationGenerated"
	ReasonGatewayNotReady             = "GatewayNotReady"
	ReasonGatewayReady                = "GatewayReady"
	ReasonAgentNotReady               = "AgentNotReady"
	ReasonAgentReady                  = "AgentReady"
	ReasonAllDataDropped              = "AllTelemetryDataDropped"
	ReasonSomeDataDropped             = "SomeTelemetryDataDropped"
	ReasonBufferFillingUp             = "BufferFillingUp"
	ReasonGatewayThrottling           = "GatewayThrottling"
	ReasonFlowHealthy                 = "Healthy"
	ReasonTLSCertificateAboutToExpire = "TLSCertificateAboutToExpire"
	ReasonTLSCertificateExpired       = "TLSCertificateExpired"
	ReasonTLSCertificateInvalid       = "TLSCertificateInvalid"

	// LogPipeline only reasons
	ReasonNoLogsDelivered       = "NoLogsDelivered"
	ReasonUnsupportedLokiOutput = "UnsupportedLokiOutput"

	// MetricPipeline only reasons
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
	ReasonNoPipelineDeployed:          "No pipelines have been deployed",
	ReasonReferencedSecretMissing:     "One or more referenced Secrets are missing",
	ReasonMaxPipelinesExceeded:        "Maximum pipeline count limit exceeded",
	ReasonTLSCertificateAboutToExpire: "TLS certificate is about to expire, configured certificate is valid until %s",
	ReasonTLSCertificateExpired:       "TLS certificate expired on %s",
	ReasonTLSCertificateInvalid:       "TLS certificate invalid: %s",
}

var metricPipelineMessages = map[string]string{
	ReasonGatewayNotReady:   "Metric gateway Deployment is not ready",
	ReasonGatewayReady:      "Metric gateway Deployment is ready",
	ReasonAgentNotReady:     "Metric agent DaemonSet is not ready",
	ReasonAgentReady:        "Metric agent DaemonSet is ready",
	ReasonComponentsRunning: "All metric components are running",
	ReasonAllDataDropped:    "All metrics dropped: backend unreachable or rejecting",
	ReasonSomeDataDropped:   "Some metrics dropped: backend unreachable or rejecting",
	ReasonBufferFillingUp:   "Buffer nearing capacity: incoming metric rate exceeds export rate",
	ReasonGatewayThrottling: "Metric gateway experiencing high influx: unable to receive metrics at current rate",
	ReasonFlowHealthy:       "No problems detected in the metric flow",
}

var tracePipelineMessages = map[string]string{
	ReasonGatewayNotReady:                "Trace gateway Deployment is not ready",
	ReasonGatewayReady:                   "Trace gateway Deployment is ready",
	ReasonTraceGatewayDeploymentNotReady: "Trace gateway Deployment is not ready",
	ReasonTraceGatewayDeploymentReady:    "Trace gateway Deployment is ready",
	ReasonComponentsRunning:              "All trace components are running",
	ReasonAllDataDropped:                 "All traces dropped: backend unreachable or rejecting",
	ReasonSomeDataDropped:                "Some traces dropped: backend unreachable or rejecting",
	ReasonBufferFillingUp:                "Buffer nearing capacity: incoming trace rate exceeds export rate",
	ReasonGatewayThrottling:              "Trace gateway experiencing high influx: unable to receive traces at current rate",
	ReasonFlowHealthy:                    "No problems detected in the trace flow",
}

var logPipelineMessages = map[string]string{
	ReasonAgentNotReady:         "Fluent Bit agent DaemonSet is not ready",
	ReasonAgentReady:            "Fluent Bit agent DaemonSet is ready",
	ReasonFluentBitDSNotReady:   "Fluent Bit DaemonSet is not ready",
	ReasonFluentBitDSReady:      "Fluent Bit DaemonSet is ready",
	ReasonUnsupportedLokiOutput: "grafana-loki output is not supported anymore. For integration with a custom Loki installation, use the `custom` output and follow https://kyma-project.io/#/telemetry-manager/user/integration/loki/README",
	ReasonComponentsRunning:     "All log components are running",
	ReasonAllDataDropped:        "All logs dropped: backend unreachable or rejecting",
	ReasonSomeDataDropped:       "Some logs dropped: backend unreachable or rejecting",
	ReasonBufferFillingUp:       "Buffer nearing capacity: incoming log rate exceeds export rate",
	ReasonNoLogsDelivered:       "No logs delivered to backend",
	ReasonFlowHealthy:           "No problems detected in the log flow",
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
