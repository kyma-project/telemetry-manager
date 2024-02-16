package logparser

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
)

func (r *Reconciler) updateStatus(ctx context.Context, parserName string) error {
	log := logf.FromContext(ctx)
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

	fluentBitReady, err := r.prober.IsReady(ctx, r.config.DaemonSet)
	if err != nil {
		return err
	}

	if fluentBitReady {
		existingPending := meta.FindStatusCondition(parser.Status.Conditions, conditions.TypePending)
		if existingPending != nil {
			newPending := conditions.New(
				conditions.TypePending,
				existingPending.Reason,
				metav1.ConditionFalse,
				parser.Generation,
			)
			meta.SetStatusCondition(&parser.Status.Conditions, newPending)
		}

		running := conditions.New(
			conditions.TypeRunning,
			conditions.ReasonFluentBitDSReady,
			metav1.ConditionTrue,
			parser.Generation,
		)
		meta.SetStatusCondition(&parser.Status.Conditions, running)

		return updateStatus(ctx, r.Client, &parser)
	}

	pending := conditions.New(
		conditions.TypePending,
		conditions.ReasonFluentBitDSNotReady,
		metav1.ConditionTrue,
		parser.Generation,
	)

	if meta.FindStatusCondition(parser.Status.Conditions, conditions.TypeRunning) != nil {
		log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s. Removing the Running condition", parser.Name, pending.Type))
		meta.RemoveStatusCondition(&parser.Status.Conditions, conditions.TypeRunning)
	}

	meta.SetStatusCondition(&parser.Status.Conditions, pending)
	return updateStatus(ctx, r.Client, &parser)
}

func updateStatus(ctx context.Context, client client.Client, parser *telemetryv1alpha1.LogParser) error {
	if err := client.Status().Update(ctx, parser); err != nil {
		return fmt.Errorf("failed to update LogParser status: %w", err)
	}
	return nil
}
