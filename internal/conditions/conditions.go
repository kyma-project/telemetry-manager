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
	ReasonNoPipelineDeployed           = "NoPipelineDeployed"
	ReasonReferencedSecretMissing      = "ReferencedSecretMissing"
	ReasonMaxPipelinesExceeded         = "MaxPipelinesExceeded"
	ReasonResourceBlocksDeletion       = "ResourceBlocksDeletion"
	ReasonConfigurationGenerated       = "ConfigurationGenerated"
	ReasonDeploymentNotReady           = "DeploymentNotReady"
	ReasonDeploymentReady              = "DeploymentReady"
	ReasonDaemonSetNotReady            = "DaemonSetNotReady"
	ReasonDaemonSetReady               = "DaemonSetReady"
	ReasonAllDataDropped               = "AllTelemetryDataDropped"
	ReasonSomeDataDropped              = "SomeTelemetryDataDropped"
	ReasonBufferFillingUp              = "BufferFillingUp"
	ReasonGatewayThrottling            = "GatewayThrottling"
	ReasonNoLogsDelivered              = "NoLogsDelivered"
	ReasonFlowHealthy                  = "Healthy"
	ReasonTLSCertificateInvalid        = "TLSCertificateInvalid"
	ReasonTLSPrivateKeyInvalid         = "TLSPrivateKeyInvalid"
	ReasonTLSCertificateExpired        = "TLSCertificateExpired"
	ReasonTLSCertificateAboutToExpire  = "TLSCertificateAboutToExpire"
	ReasonTLSCertificateKeyPairInvalid = "TLSCertificateKeyPairInvalid"

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
	ReasonNoPipelineDeployed:           "No pipelines have been deployed",
	ReasonReferencedSecretMissing:      "One or more referenced Secrets are missing",
	ReasonMaxPipelinesExceeded:         "Maximum pipeline count limit exceeded",
	ReasonTLSCertificateInvalid:        "TLS certificate invalid: %s",
	ReasonTLSPrivateKeyInvalid:         "TLS private key invalid: %s",
	ReasonTLSCertificateExpired:        "TLS certificate expired on %s",
	ReasonTLSCertificateAboutToExpire:  "TLS certificate is about to expire, configured certificate is valid until %s",
	ReasonTLSCertificateKeyPairInvalid: "TLS certificate and private key do not match: %s",
}

var metricPipelineMessages = map[string]string{
	ReasonDeploymentNotReady:      "Metric gateway Deployment is not ready",
	ReasonDeploymentReady:         "Metric gateway Deployment is ready",
	ReasonDaemonSetNotReady:       "Metric agent DaemonSet is not ready",
	ReasonDaemonSetReady:          "Metric agent DaemonSet is ready",
	ReasonMetricComponentsRunning: "All metric components are running",
	ReasonAllDataDropped:          "All metrics dropped: backend unreachable or rejecting",
	ReasonSomeDataDropped:         "Some metrics dropped: backend unreachable or rejecting",
	ReasonBufferFillingUp:         "Buffer nearing capacity: incoming metric rate exceeds export rate",
	ReasonGatewayThrottling:       "Metric gateway experiencing high influx: unable to receive metrics at current rate",
	ReasonFlowHealthy:             "No problems detected in the metric flow",
}

var tracePipelineMessages = map[string]string{
	ReasonDeploymentNotReady:             "Trace gateway Deployment is not ready",
	ReasonDeploymentReady:                "Trace gateway Deployment is ready",
	ReasonTraceGatewayDeploymentNotReady: "Trace gateway Deployment is not ready",
	ReasonTraceGatewayDeploymentReady:    "Trace gateway Deployment is ready",
	ReasonTraceComponentsRunning:         "All trace components are running",
	ReasonAllDataDropped:                 "All traces dropped: backend unreachable or rejecting",
	ReasonSomeDataDropped:                "Some traces dropped: backend unreachable or rejecting",
	ReasonBufferFillingUp:                "Buffer nearing capacity: incoming trace rate exceeds export rate",
	ReasonGatewayThrottling:              "Trace gateway experiencing high influx: unable to receive traces at current rate",
	ReasonFlowHealthy:                    "No problems detected in the trace flow",
}

var logPipelineMessages = map[string]string{
	ReasonDaemonSetNotReady:     "Fluent Bit DaemonSet is not ready",
	ReasonDaemonSetReady:        "Fluent Bit DaemonSet is ready",
	ReasonFluentBitDSNotReady:   "Fluent Bit DaemonSet is not ready",
	ReasonFluentBitDSReady:      "Fluent Bit DaemonSet is ready",
	ReasonUnsupportedLokiOutput: "grafana-loki output is not supported anymore. For integration with a custom Loki installation, use the `custom` output and follow https://kyma-project.io/#/telemetry-manager/user/integration/loki/README",
	ReasonLogComponentsRunning:  "All log components are running",
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
