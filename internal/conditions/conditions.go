package conditions

const (
	ReasonNoPipelineDeployed      = "NoPipelineDeployed"
	ReasonReferencedSecretMissing = "ReferencedSecretMissing"
	ReasonWaitingForLock          = "WaitingForLock"
	ReasonResourceBlocksDeletion  = "ResourceBlocksDeletion"

	ReasonFluentBitDSNotReady = "FluentBitDaemonSetNotReady"
	ReasonFluentBitDSReady    = "FluentBitDaemonSetReady"

	ReasonMetricGatewayDeploymentNotReady = "MetricGatewayDeploymentNotReady"
	ReasonMetricGatewayDeploymentReady    = "MetricGatewayDeploymentReady"

	ReasonTraceGatewayDeploymentNotReady = "TraceGatewayDeploymentNotReady"
	ReasonTraceGatewayDeploymentReady    = "TraceGatewayDeploymentReady"
)

var message = map[string]string{
	ReasonNoPipelineDeployed:      "No pipelines have been deployed",
	ReasonReferencedSecretMissing: "One or more referenced Secrets are missing",
	ReasonWaitingForLock:          "Waiting for the lock",

	ReasonFluentBitDSNotReady: "Fluent Bit DaemonSet is not ready",
	ReasonFluentBitDSReady:    "Fluent Bit DaemonSet is ready",

	ReasonMetricGatewayDeploymentNotReady: "Metric gateway Deployment is not ready",
	ReasonMetricGatewayDeploymentReady:    "Metric gateway Deployment is ready",

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
