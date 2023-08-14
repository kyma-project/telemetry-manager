package reconciler

const (
	ReasonNoPipelineDeployed           = "NoPipelineDeployed"
	ReasonFluentBitDSNotReady          = "FluentBitDaemonSetNotReady"
	ReasonFluentBitDSReady             = "FluentBitDaemonSetReady"
	ReasonReferencedSecretMissing      = "ReferencedSecretMissing"
	ReasonFluentBitPodCrashBackLooping = "FluentBitPodCrashBackLoop"

	ReasonMetricGatewayDeploymentNotReady = "MetricGatewayDeploymentNotReady"
	ReasonMetricGatewayDeploymentReady    = "MetricGatewayDeploymentReady"
	ReasonReferencedSecretMissingReason   = "ReferencedSecretMissing"
	ReasonWaitingForLock                  = "WaitingForLock"

	ReasonTraceCollectorDeploymentNotReady = "TraceCollectorDeploymentNotReady"
	ReasonTraceCollectorDeploymentReady    = "TraceCollectorDeploymentReady"

	ReasonPodStatusUnknown   = "PodStatusUnknown"
	ReasonDeploymentReady    = "DeploymentReady"
	ReasonDeploymentNotReady = "DeploymentNotReady"
)

var Conditions = map[string]string{
	ReasonNoPipelineDeployed: "No pipelines have been deployed",

	ReasonFluentBitDSNotReady:          "Fluent bit Daemonset is not ready",
	ReasonFluentBitDSReady:             "Fluent bit Daemonset is ready",
	ReasonReferencedSecretMissing:      "One or more referenced secrets are missing",
	ReasonFluentBitPodCrashBackLooping: "Fluent bit pod is in crashback loop",

	ReasonMetricGatewayDeploymentNotReady: "Metric gateway deployment is not ready",
	ReasonMetricGatewayDeploymentReady:    "Metric gateway deployment is ready",
	ReasonWaitingForLock:                  "Waiting for the lock",

	ReasonTraceCollectorDeploymentNotReady: "Trace collector is deployment not ready",
	ReasonTraceCollectorDeploymentReady:    "Trace collector deployment is ready",

	ReasonPodStatusUnknown:   "Pod status is unknown",
	ReasonDeploymentReady:    "Deployment is ready",
	ReasonDeploymentNotReady: "Deployment is not ready",
}
