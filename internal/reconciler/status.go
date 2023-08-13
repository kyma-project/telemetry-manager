package reconciler

const (
	ReasonNoPipelineDeployed               = "NoPipelineDeployed"
	ReasonFluentBitDSNotReady              = "FluentBitDaemonSetNotReady"
	ReasonFluentBitDSReady                 = "FluentBitDaemonSetReady"
	ReasonReferencedSecretMissing          = "ReferencedSecretMissing"
	ReasonFluentBitPodCrashBackLooping     = "FluentBitPodCrashBackLoop"
	ReasonMetricGatewayDeploymentNotReady  = "MetricGatewayDeploymentNotReady"
	ReasonMetricGatewayDeploymentReady     = "MetricGatewayDeploymentReady"
	ReasonReferencedSecretMissingReason    = "ReferencedSecretMissing"
	ReasonWaitingForLock                   = "WaitingForLock"
	ReasonTraceCollectorDeploymentNotReady = "TraceCollectorDeploymentNotReady"
	ReasonTraceCollectorDeploymentReady    = "TraceCollectorDeploymentReady"
	ReasonPodStatusUnknown                 = "PodStatusUnknown"
	ReasonDeploymentReady                  = "DeploymentReady"
	ReasonDeploymentNotReady               = "DeploymentNotReady"
)
