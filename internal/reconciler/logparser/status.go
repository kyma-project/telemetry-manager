package logparser

import (
	"context"
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	fluentBitReady, err := r.prober.IsReady(ctx, r.config.DaemonSet)
	if err != nil {
		return err
	}

	if !fluentBitReady {
		conditions.SetPendingCondition(ctx, &parser.Status.Conditions, parser.Generation, conditions.ReasonFluentBitDSNotReady, parser.Name)
		return updateStatus(ctx, r.Client, &parser)

	}

	conditions.SetRunningCondition(ctx, &parser.Status.Conditions, parser.Generation, conditions.ReasonFluentBitDSReady, parser.Name)
	return updateStatus(ctx, r.Client, &parser)
}

func updateStatus(ctx context.Context, client client.Client, parser *telemetryv1alpha1.LogParser) error {
	if err := client.Status().Update(ctx, parser); err != nil {
		return fmt.Errorf("failed to update LogParser status: %w", err)
	}
	return nil
}
