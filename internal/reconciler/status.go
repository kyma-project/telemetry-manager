package reconciler

const (
	ReasonNoPipelineDeployed      = "NoPipelineDeployed"
	ReasonReferencedSecretMissing = "ReferencedSecretMissing"
	ReasonWaitingForLock          = "WaitingForLock"

	ReasonFluentBitDSNotReady          = "FluentBitDaemonSetNotReady"
	ReasonFluentBitDSReady             = "FluentBitDaemonSetReady"
	ReasonFluentBitPodCrashBackLooping = "FluentBitPodCrashBackLoop"

	ReasonMetricGatewayDeploymentNotReady = "MetricGatewayDeploymentNotReady"
	ReasonMetricGatewayDeploymentReady    = "MetricGatewayDeploymentReady"

	ReasonTraceGatewayDeploymentNotReady = "TraceCollectorDeploymentNotReady"
	ReasonTraceGatewayDeploymentReady    = "TraceCollectorDeploymentReady"
)

var Conditions = map[string]string{
	ReasonNoPipelineDeployed:      "No pipelines have been deployed",
	ReasonReferencedSecretMissing: "One or more referenced secrets are missing",
	ReasonWaitingForLock:          "Waiting for the lock",

	ReasonFluentBitDSNotReady:          "Fluent bit Daemonset is not ready",
	ReasonFluentBitDSReady:             "Fluent bit Daemonset is ready",
	ReasonFluentBitPodCrashBackLooping: "Fluent bit pod is in crashback loop",

	ReasonMetricGatewayDeploymentNotReady: "Metric gateway deployment is not ready",
	ReasonMetricGatewayDeploymentReady:    "Metric gateway deployment is ready",

	ReasonTraceGatewayDeploymentNotReady: "Trace collector is deployment not ready",
	ReasonTraceGatewayDeploymentReady:    "Trace collector deployment is ready",
}
