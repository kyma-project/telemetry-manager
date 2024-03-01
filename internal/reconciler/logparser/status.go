package logparser

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
)

func (r *Reconciler) updateStatus(ctx context.Context, parserName string) error {
	var parser telemetryv1alpha1.LogParser
	if err := r.Get(ctx, types.NamespacedName{Name: parserName}, &parser); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to get LogParser: %v", err)
	}

	if parser.DeletionTimestamp != nil {
		return nil
	}

	// If the "AgentHealthy" type doesn't exist in the conditions,
	// then we need to reset the conditions list to ensure that the "Pending" and "Running" conditions are appended to the end of the conditions list
	// Check step 3 in https://github.com/kyma-project/telemetry-manager/blob/main/docs/contributor/arch/004-consolidate-pipeline-statuses.md#decision
	if meta.FindStatusCondition(parser.Status.Conditions, conditions.TypeAgentHealthy) == nil {
		parser.Status.Conditions = []metav1.Condition{}
	}

	r.setAgentHealthyCondition(ctx, &parser)
	r.setPendingAndRunningConditions(ctx, &parser)

	if err := r.Status().Update(ctx, &parser); err != nil {
		return fmt.Errorf("failed to update LogParser status: %w", err)
	}
	return nil

}

func (r *Reconciler) setAgentHealthyCondition(ctx context.Context, parser *telemetryv1alpha1.LogParser) {
	healthy, err := r.prober.IsReady(ctx, r.config.DaemonSet)
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to probe fluent bit daemonset - set condition as not healthy")
		healthy = false
	}

	status := metav1.ConditionFalse
	reason := conditions.ReasonDaemonSetNotReady
	if healthy {
		status = metav1.ConditionTrue
		reason = conditions.ReasonDaemonSetReady
	}

	meta.SetStatusCondition(&parser.Status.Conditions, conditions.New(conditions.TypeAgentHealthy, reason, status, parser.Generation, conditions.LogsMessage))
}

func (r *Reconciler) setPendingAndRunningConditions(ctx context.Context, parser *telemetryv1alpha1.LogParser) {
	fluentBitReady, err := r.prober.IsReady(ctx, r.config.DaemonSet)
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to probe fluent bit daemonset")
		fluentBitReady = false
	}

	if !fluentBitReady {
		conditions.SetPendingCondition(ctx, &parser.Status.Conditions, parser.Generation, conditions.ReasonFluentBitDSNotReady, parser.Name, conditions.LogsMessage)
		return

	}

	conditions.SetRunningCondition(ctx, &parser.Status.Conditions, parser.Generation, conditions.ReasonFluentBitDSReady, parser.Name, conditions.LogsMessage)
}
