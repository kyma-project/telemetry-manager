package conditions

const (
	ReasonNoPipelineDeployed      = "NoPipelineDeployed"
	ReasonReferencedSecretMissing = "ReferencedSecretMissing"
	ReasonWaitingForLock          = "WaitingForLock"

	ReasonFluentBitDSNotReady       = "FluentBitDaemonSetNotReady"
	ReasonFluentBitDSReady          = "FluentBitDaemonSetReady"
	ReasonLogResourceBlocksDeletion = "LogResourceBlocksDeletion"

	ReasonMetricGatewayDeploymentNotReady = "MetricGatewayDeploymentNotReady"
	ReasonMetricGatewayDeploymentReady    = "MetricGatewayDeploymentReady"
	ReasonMetricResourceBlocksDeletion    = "MetricResourceBlocksDeletion"

	ReasonTraceGatewayDeploymentNotReady = "TraceGatewayDeploymentNotReady"
	ReasonTraceGatewayDeploymentReady    = "TraceGatewayDeploymentReady"
	ReasonTraceResourceBlocksDeletion    = "TraceResourceBlocksDeletion"
)

var message = map[string]string{
	ReasonNoPipelineDeployed:      "No pipelines have been deployed",
	ReasonReferencedSecretMissing: "One or more referenced Secrets are missing",
	ReasonWaitingForLock:          "Waiting for the lock",

	ReasonFluentBitDSNotReady:       "Fluent Bit DaemonSet is not ready",
	ReasonFluentBitDSReady:          "Fluent Bit DaemonSet is ready",
	ReasonLogResourceBlocksDeletion: "One or more LogPipelines/LogParsers still exist",

	ReasonMetricGatewayDeploymentNotReady: "Metric gateway Deployment is not ready",
	ReasonMetricGatewayDeploymentReady:    "Metric gateway Deployment is ready",
	ReasonMetricResourceBlocksDeletion:    "One or more MetricPipelines still exist",

	ReasonTraceGatewayDeploymentNotReady: "Trace gateway Deployment is not ready",
	ReasonTraceGatewayDeploymentReady:    "Trace gateway Deployment is ready",
	ReasonTraceResourceBlocksDeletion:    "One or more TracePipelines still exist",
}

// CommonMessageFor returns a human-readable message corresponding to a given reason.
// In more advanced scenarios, you may craft custom messages tailored to specific use cases.
func CommonMessageFor(reason string) string {
	if condMessage, found := message[reason]; found {
		return condMessage
	}
	return ""
}
