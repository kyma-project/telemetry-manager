package reconciler

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
}

const (
	LogConditionType    = "Logging"
	MetricConditionType = "Metrics"
	TraceConditionType  = "Tracing"
)

const (
	ConditionStatusTrue    metav1.ConditionStatus = "True"
	ConditionStatusFalse   metav1.ConditionStatus = "False"
	ConditionStatusUnknown metav1.ConditionStatus = "Unknown"
)
