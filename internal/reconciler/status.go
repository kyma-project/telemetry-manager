package reconciler

const (
	ReasonNoPipelineDeployed      = "NoPipelineDeployed"
	ReasonReferencedSecretMissing = "ReferencedSecretMissing"
	ReasonWaitingForLock          = "WaitingForLock"

	ReasonFluentBitDSNotReady          = "FluentBitDaemonSetNotReady"
	ReasonFluentBitDSReady             = "FluentBitDaemonSetReady"
	ReasonLogComponentsDeletionBlocked = "LogComponentsDeletionBlocked"

	ReasonMetricGatewayDeploymentNotReady = "MetricGatewayDeploymentNotReady"
	ReasonMetricGatewayDeploymentReady    = "MetricGatewayDeploymentReady"
	ReasonMetricComponentsDeletionBlocked = "MetricComponentsDeletionBlocked"

	ReasonTraceGatewayDeploymentNotReady = "TraceGatewayDeploymentNotReady"
	ReasonTraceGatewayDeploymentReady    = "TraceGatewayDeploymentReady"
	ReasonTraceComponentsDeletionBlocked = "TraceComponentsDeletionBlocked"
)

var conditions = map[string]string{
	ReasonNoPipelineDeployed:      "No pipelines have been deployed",
	ReasonReferencedSecretMissing: "One or more referenced Secrets are missing",
	ReasonWaitingForLock:          "Waiting for the lock",

	ReasonFluentBitDSNotReady:          "Fluent Bit DaemonSet is not ready",
	ReasonFluentBitDSReady:             "Fluent Bit DaemonSet is ready",
	ReasonLogComponentsDeletionBlocked: "One or more LogPipelines/LogParsers still exist",

	ReasonMetricGatewayDeploymentNotReady: "Metric gateway Deployment is not ready",
	ReasonMetricGatewayDeploymentReady:    "Metric gateway Deployment is ready",
	ReasonMetricComponentsDeletionBlocked: "One or more MetricPipelines still exist",

	ReasonTraceGatewayDeploymentNotReady: "Trace gateway Deployment is not ready",
	ReasonTraceGatewayDeploymentReady:    "Trace gateway Deployment is ready",
	ReasonTraceComponentsDeletionBlocked: "One or more TracePipelines still exist",
}

func Condition(reason string) string {
	if cond, found := conditions[reason]; found {
		return cond
	}
	return ""
}
