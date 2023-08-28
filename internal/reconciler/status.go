package reconciler

const (
	ReasonNoPipelineDeployed      = "NoPipelineDeployed"
	ReasonReferencedSecretMissing = "ReferencedSecretMissing"
	ReasonWaitingForLock          = "WaitingForLock"

	ReasonFluentBitDSNotReady = "FluentBitDaemonSetNotReady"
	ReasonFluentBitDSReady    = "FluentBitDaemonSetReady"

	ReasonMetricGatewayDeploymentNotReady = "MetricGatewayDeploymentNotReady"
	ReasonMetricGatewayDeploymentReady    = "MetricGatewayDeploymentReady"

	ReasonTraceGatewayDeploymentNotReady = "TraceCollectorDeploymentNotReady"
	ReasonTraceGatewayDeploymentReady    = "TraceCollectorDeploymentReady"
)

var conditions = map[string]string{
	ReasonNoPipelineDeployed:      "No pipelines have been deployed",
	ReasonReferencedSecretMissing: "One or more referenced Secrets are missing",
	ReasonWaitingForLock:          "Waiting for the lock",

	ReasonFluentBitDSNotReady: "Fluent Bit DaemonSet is not ready",
	ReasonFluentBitDSReady:    "Fluent Bit DaemonSet is ready",

	ReasonMetricGatewayDeploymentNotReady: "Metric gateway Deployment is not ready",
	ReasonMetricGatewayDeploymentReady:    "Metric gateway Deployment is ready",

	ReasonTraceGatewayDeploymentNotReady: "Trace collector Deployment is not ready",
	ReasonTraceGatewayDeploymentReady:    "Trace collector Deployment is ready",
}

func Condition(reason string) string {
	if cond, found := conditions[reason]; found {
		return cond
	}
	return ""
}
