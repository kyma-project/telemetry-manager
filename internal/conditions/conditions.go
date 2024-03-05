package conditions

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	TypeGatewayHealthy         = "GatewayHealthy"
	TypeAgentHealthy           = "AgentHealthy"
	TypeConfigurationGenerated = "ConfigurationGenerated"

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
	ReasonNoPipelineDeployed      = "NoPipelineDeployed"
	ReasonReferencedSecretMissing = "ReferencedSecretMissing"
	ReasonMaxPipelinesExceeded    = "MaxPipelinesExceeded"
	ReasonResourceBlocksDeletion  = "ResourceBlocksDeletion"
	ReasonConfigurationGenerated  = "ConfigurationGenerated"
	ReasonDeploymentNotReady      = "DeploymentNotReady"
	ReasonDeploymentReady         = "DeploymentReady"
	ReasonDaemonSetNotReady       = "DaemonSetNotReady"
	ReasonDaemonSetReady          = "DaemonSetReady"

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

var commonMessage = map[string]string{
	ReasonNoPipelineDeployed:      "No pipelines have been deployed",
	ReasonReferencedSecretMissing: "One or more referenced Secrets are missing",
	ReasonMaxPipelinesExceeded:    "Maximum pipeline count limit exceeded",
}

var MetricsMessage = map[string]string{
	ReasonDeploymentNotReady:      "Metric gateway Deployment is not ready",
	ReasonDeploymentReady:         "Metric gateway Deployment is ready",
	ReasonDaemonSetNotReady:       "Metric agent DaemonSet is not ready",
	ReasonDaemonSetReady:          "Metric agent DaemonSet is ready",
	ReasonMetricComponentsRunning: "All metric components are running",
}

var TracesMessage = map[string]string{
	ReasonDeploymentNotReady:             "Trace gateway Deployment is not ready",
	ReasonDeploymentReady:                "Trace gateway Deployment is ready",
	ReasonTraceGatewayDeploymentNotReady: "Trace gateway Deployment is not ready",
	ReasonTraceGatewayDeploymentReady:    "Trace gateway Deployment is ready",
	ReasonTraceComponentsRunning:         "All trace components are running",
}

var LogsMessage = map[string]string{
	ReasonDaemonSetNotReady:     "Fluent Bit DaemonSet is not ready",
	ReasonDaemonSetReady:        "Fluent Bit DaemonSet is ready",
	ReasonFluentBitDSNotReady:   "Fluent Bit DaemonSet is not ready",
	ReasonFluentBitDSReady:      "Fluent Bit DaemonSet is ready",
	ReasonUnsupportedLokiOutput: "grafana-loki output is not supported anymore. For integration with a custom Loki installation, use the `custom` output and follow https://kyma-project.io/#/telemetry-manager/user/integration/loki/README",
	ReasonLogComponentsRunning:  "All log components are running",
}

func New(condType, reason string, status metav1.ConditionStatus, generation int64, messageMap map[string]string) metav1.Condition {
	return metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            MessageFor(reason, messageMap),
		ObservedGeneration: generation,
	}
}

// MessageFor returns a human-readable message corresponding to a given reason.
// In more advanced scenarios, you may craft custom messages tailored to specific use cases.
func MessageFor(reason string, messageMap map[string]string) string {
	if condMessage, found := commonMessage[reason]; found {
		return condMessage
	}
	if condMessage, found := messageMap[reason]; found {
		return condMessage
	}
	return ""
}

func SetPendingCondition(ctx context.Context, conditions *[]metav1.Condition, generation int64, reason, resourceName string, messageMap map[string]string) {
	log := logf.FromContext(ctx)

	pending := New(
		TypePending,
		reason,
		metav1.ConditionTrue,
		generation,
		messageMap,
	)
	pending.Message = PendingTypeDeprecationMsg + pending.Message

	if meta.FindStatusCondition(*conditions, TypeRunning) != nil {
		log.V(1).Info(fmt.Sprintf("Updating the status of %s: Removing the Running condition", resourceName))
		meta.RemoveStatusCondition(conditions, TypeRunning)
	}

	log.V(1).Info(fmt.Sprintf("Updating the status of %s: Setting the Pending condition to True", resourceName))
	meta.SetStatusCondition(conditions, pending)
}

func SetRunningCondition(ctx context.Context, conditions *[]metav1.Condition, generation int64, reason, resourceName string, messageMap map[string]string) {
	log := logf.FromContext(ctx)

	existingPending := meta.FindStatusCondition(*conditions, TypePending)
	if existingPending != nil {
		newPending := New(
			TypePending,
			existingPending.Reason,
			metav1.ConditionFalse,
			generation,
			messageMap,
		)
		newPending.Message = PendingTypeDeprecationMsg + newPending.Message
		log.V(1).Info(fmt.Sprintf("Updating the status of %s: Setting the Pending condition to False", resourceName))
		meta.SetStatusCondition(conditions, newPending)
	}

	running := New(
		TypeRunning,
		reason,
		metav1.ConditionTrue,
		generation,
		messageMap,
	)
	running.Message = RunningTypeDeprecationMsg + running.Message

	log.V(1).Info(fmt.Sprintf("Updating the status of %s: Setting the Running condition to True", resourceName))
	meta.SetStatusCondition(conditions, running)
}
