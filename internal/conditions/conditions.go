package conditions

import "strings"

const (
	LinkNoDataArriveAtBackend     = "https://kyma-project.io/external-content/telemetry-manager/docs/user/troubleshooting.html#no-data-arrive-at-the-backend"
	LinkNotAllDataArriveAtBackend = "https://kyma-project.io/external-content/telemetry-manager/docs/user/troubleshooting.html#not-all-data-arrive-at-the-backend"
	LinkGatewayThrottling         = "https://kyma-project.io/external-content/telemetry-manager/docs/user/troubleshooting.html#gateway-throttling"
	LinkOTTLSpecInvalid           = "https://kyma-project.io/external-content/telemetry-manager/docs/user/troubleshooting.html#ottl-spec-invalid-with-unspecific-error-message"

	LinkFluentBitNoLogsArriveAtBackend     = "https://kyma-project.io/#/telemetry-manager/user/02-logs?id=no-logs-arrive-at-the-backend"
	LinkFluentBitNotAllLogsArriveAtBackend = "https://kyma-project.io/#/telemetry-manager/user/02-logs?id=not-all-logs-arrive-at-the-backend"
	LinkFluenBitBufferFillingUp            = "https://kyma-project.io/#/telemetry-manager/user/02-logs?id=agent-buffer-filling-up"
)

const (
	TypeAgentHealthy            = "AgentHealthy"
	TypeConfigurationGenerated  = "ConfigurationGenerated"
	TypeFlowHealthy             = "TelemetryFlowHealthy"
	TypeGatewayHealthy          = "GatewayHealthy"
	TypeLogComponentsHealthy    = "LogComponentsHealthy"
	TypeMetricComponentsHealthy = "MetricComponentsHealthy"
	TypeTraceComponentsHealthy  = "TraceComponentsHealthy"
)

const (
	// Common reasons

	ReasonAgentNotReady                 = "AgentNotReady"
	ReasonAgentReady                    = "AgentReady"
	ReasonEndpointInvalid               = "EndpointInvalid"
	ReasonGatewayConfigured             = "GatewayConfigured"
	ReasonGatewayNotReady               = "GatewayNotReady"
	ReasonGatewayReady                  = "GatewayReady"
	ReasonMaxPipelinesExceeded          = "MaxPipelinesExceeded"
	ReasonReferencedSecretMissing       = "ReferencedSecretMissing"
	ReasonSelfMonFlowHealthy            = "FlowHealthy"
	ReasonSelfMonGatewayAllDataDropped  = "GatewayAllTelemetryDataDropped"
	ReasonSelfMonAgentAllDataDropped    = "AgentAllTelemetryDataDropped"
	ReasonSelfMonGatewaySomeDataDropped = "GatewaySomeTelemetryDataDropped"
	ReasonSelfMonAgentSomeDataDropped   = "AgentSomeTelemetryDataDropped"
	ReasonSelfMonAgentBufferFillingUp   = "AgentBufferFillingUp"
	ReasonSelfMonGatewayProbingFailed   = "GatewayProbingFailed"
	ReasonSelfMonAgentProbingFailed     = "AgentProbingFailed"
	ReasonSelfMonGatewayThrottling      = "GatewayThrottling"
	ReasonSelfMonConfigNotGenerated     = "ConfigurationNotGenerated"
	ReasonTLSCertificateAboutToExpire   = "TLSCertificateAboutToExpire"
	ReasonTLSCertificateExpired         = "TLSCertificateExpired"
	ReasonTLSConfigurationInvalid       = "TLSConfigurationInvalid"
	ReasonValidationFailed              = "ValidationFailed"
	ReasonRolloutInProgress             = "RolloutInProgress"
	ReasonOTTLSpecInvalid               = "OTTLSpecInvalid"

	// Telemetry reasons

	ReasonComponentsRunning      = "ComponentsRunning"
	ReasonNoPipelineDeployed     = "NoPipelineDeployed"
	ReasonResourceBlocksDeletion = "ResourceBlocksDeletion"

	// LogPipeline reasons

	ReasonNoFluentbitInFipsMode       = "FipsModeEnabled"
	ReasonAgentConfigured             = "AgentConfigured"
	ReasonSelfMonAgentNoLogsDelivered = "AgentNoLogsDelivered"
	ReasonLogAgentNotRequired         = "AgentNotRequired"

	// MetricPipeline reasons

	ReasonMetricAgentNotRequired = "AgentNotRequired"
)

// Error messages
const (
	podIsNotScheduled    = "Pod is not scheduled: %s"
	podIsPending         = "Pod is in the pending state because container: %s is not running due to: %s. Please check the container: %s logs."
	podIsFailed          = "Pod is in the failed state due to: %s"
	podRolloutInProgress = "Pods are being started/updated"
)

var commonMessages = map[string]string{
	ReasonNoPipelineDeployed:          "No pipelines have been deployed",
	ReasonSelfMonFlowHealthy:          "No problems detected in the telemetry flow",
	ReasonSelfMonGatewayProbingFailed: "Could not determine the health of the telemetry flow because the self monitor probing of gateway failed",
	ReasonSelfMonAgentProbingFailed:   "Could not determine the health of the telemetry flow because the self monitor probing of agent failed",
	ReasonTLSConfigurationInvalid:     "TLS configuration invalid: %s",
	ReasonValidationFailed:            "Pipeline validation failed due to an error from the Kubernetes API server",
	ReasonOTTLSpecInvalid:             "OTTL specification is invalid, %s. Fix the syntax error indicated by the message or see troubleshooting: " + LinkOTTLSpecInvalid,
}

var commonLogPipelineMessages = map[string]string{
	ReasonAgentConfigured:       "LogPipeline specification is successfully applied to the configuration of Log agent",
	ReasonAgentNotReady:         "Log agent DaemonSet is not ready",
	ReasonAgentReady:            "Log agent DaemonSet is ready",
	ReasonComponentsRunning:     "All log components are running",
	ReasonNoFluentbitInFipsMode: "HTTP/custom output types are not supported when FIPS mode is enabled",
}

var fluentBitLogPipelineMessages = map[string]string{
	ReasonEndpointInvalid: "HTTP output host invalid: %s",

	ReasonSelfMonAgentAllDataDropped:  "Backend is not reachable or rejecting logs. All logs are dropped. See troubleshooting: " + LinkFluentBitNoLogsArriveAtBackend,
	ReasonSelfMonAgentBufferFillingUp: "Buffer nearing capacity. Incoming log rate exceeds export rate. See troubleshooting: " + LinkFluenBitBufferFillingUp,
	ReasonSelfMonConfigNotGenerated:   "No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent. Check the 'ConfigurationGenerated' condition for more details",
	ReasonSelfMonAgentNoLogsDelivered: "Backend is not reachable or rejecting logs. Logs are buffered and not yet dropped. See troubleshooting: " + LinkFluentBitNoLogsArriveAtBackend,
	ReasonSelfMonAgentSomeDataDropped: "Backend is reachable, but rejecting logs. Some logs are dropped. See troubleshooting: " + LinkFluentBitNotAllLogsArriveAtBackend,
}

var otelLogPipelineMessages = map[string]string{
	ReasonEndpointInvalid: "OTLP output endpoint invalid: %s",

	ReasonGatewayConfigured: "LogPipeline specification is successfully applied to the configuration of Log gateway",
	ReasonGatewayNotReady:   "Log gateway Deployment is not ready",
	ReasonGatewayReady:      "Log gateway Deployment is ready",

	ReasonSelfMonGatewayAllDataDropped:  "Backend is not reachable or rejecting logs. All logs are dropped in Log gateway. See troubleshooting: " + LinkNoDataArriveAtBackend,
	ReasonSelfMonAgentAllDataDropped:    "Backend is not reachable or rejecting logs. All logs are dropped in Log agent. See troubleshooting: " + LinkNoDataArriveAtBackend,
	ReasonSelfMonGatewaySomeDataDropped: "Backend is reachable, but rejecting logs. Some logs are dropped in Log gateway. See troubleshooting: " + LinkNotAllDataArriveAtBackend,
	ReasonSelfMonAgentSomeDataDropped:   "Backend is reachable, but rejecting logs. Some logs are dropped in Log agent. See troubleshooting: " + LinkNotAllDataArriveAtBackend,
	ReasonSelfMonGatewayThrottling:      "Log gateway is unable to receive logs at current rate. See troubleshooting: " + LinkGatewayThrottling,
	ReasonSelfMonConfigNotGenerated:     "No logs delivered to backend because LogPipeline specification is not applied to the configuration of Log agent and Log gateway. Check the 'ConfigurationGenerated' condition for more details",
}

var tracePipelineMessages = map[string]string{
	ReasonComponentsRunning:             "All trace components are running",
	ReasonEndpointInvalid:               "OTLP output endpoint invalid: %s",
	ReasonGatewayConfigured:             "TracePipeline specification is successfully applied to the configuration of Trace gateway",
	ReasonGatewayNotReady:               "Trace gateway Deployment is not ready",
	ReasonGatewayReady:                  "Trace gateway Deployment is ready",
	ReasonSelfMonGatewayAllDataDropped:  "Backend is not reachable or rejecting spans. All spans are dropped. See troubleshooting: " + LinkNoDataArriveAtBackend,
	ReasonSelfMonConfigNotGenerated:     "No spans delivered to backend because TracePipeline specification is not applied to the configuration of Trace gateway. Check the 'ConfigurationGenerated' condition for more details",
	ReasonSelfMonGatewayThrottling:      "Trace gateway is unable to receive spans at current rate. See troubleshooting: " + LinkGatewayThrottling,
	ReasonSelfMonGatewaySomeDataDropped: "Backend is reachable, but rejecting spans. Some spans are dropped. See troubleshooting: " + LinkNotAllDataArriveAtBackend,
}

var metricPipelineMessages = map[string]string{
	ReasonAgentNotReady:                 "Metric agent DaemonSet is not ready",
	ReasonAgentReady:                    "Metric agent DaemonSet is ready",
	ReasonComponentsRunning:             "All metric components are running",
	ReasonEndpointInvalid:               "OTLP output endpoint invalid: %s",
	ReasonGatewayConfigured:             "MetricPipeline specification is successfully applied to the configuration of Metric gateway",
	ReasonGatewayNotReady:               "Metric gateway Deployment is not ready",
	ReasonGatewayReady:                  "Metric gateway Deployment is ready",
	ReasonSelfMonGatewayAllDataDropped:  "Backend is not reachable or rejecting metrics. All metrics are dropped. See troubleshooting: " + LinkNoDataArriveAtBackend,
	ReasonSelfMonAgentAllDataDropped:    "Backend is not reachable or rejecting metrics. All metrics are dropped. See troubleshooting: " + LinkNoDataArriveAtBackend,
	ReasonSelfMonConfigNotGenerated:     "No metrics delivered to backend because MetricPipeline specification is not applied to the configuration of Metric gateway. Check the 'ConfigurationGenerated' condition for more details",
	ReasonSelfMonGatewayThrottling:      "Metric gateway is unable to receive metrics at current rate. See troubleshooting: " + LinkGatewayThrottling,
	ReasonSelfMonGatewaySomeDataDropped: "Backend is reachable, but rejecting metrics. Some metrics are dropped. See troubleshooting: " + LinkNotAllDataArriveAtBackend,
	ReasonSelfMonAgentSomeDataDropped:   "Backend is reachable, but rejecting metrics. Some metrics are dropped. See troubleshooting: " + LinkNotAllDataArriveAtBackend,
}

func MessageForOtelLogPipeline(reason string) string {
	return messageForLogPipelines(reason, otelLogPipelineMessages)
}

func MessageForFluentBitLogPipeline(reason string) string {
	return messageForLogPipelines(reason, fluentBitLogPipelineMessages)
}

func messageForLogPipelines(reason string, specializedMessages map[string]string) string {
	if condMessage, found := commonMessages[reason]; found {
		return condMessage
	}

	if condMessage, found := commonLogPipelineMessages[reason]; found {
		return condMessage
	}

	if condMessage, found := specializedMessages[reason]; found {
		return condMessage
	}

	return ""
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

// ConvertErrToMsg converts the error to a condition message by capitalizing the error message
func ConvertErrToMsg(err error) string {
	errMsg := err.Error()
	return strings.ToUpper(errMsg[:1]) + errMsg[1:]
}
