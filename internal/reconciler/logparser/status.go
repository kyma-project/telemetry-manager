package logparser

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus"
)

func (r *Reconciler) updateStatus(ctx context.Context, parserName string) error {
	var parser telemetryv1alpha1.LogParser
	if err := r.Get(ctx, types.NamespacedName{Name: parserName}, &parser); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to get LogParser: %w", err)
	}

	if parser.DeletionTimestamp != nil {
		return nil
	}

	r.setAgentHealthyCondition(ctx, &parser)

	if err := r.Status().Update(ctx, &parser); err != nil {
		return fmt.Errorf("failed to update LogParser status: %w", err)
	}

	return nil
}

func (r *Reconciler) setAgentHealthyCondition(ctx context.Context, parser *telemetryv1alpha1.LogParser) {
	condition := commonstatus.GetAgentHealthyCondition(ctx, r.prober, r.config.DaemonSet, r.errorConverter, commonstatus.SignalTypeLogs)
	condition.ObservedGeneration = parser.Generation
	meta.SetStatusCondition(&parser.Status.Conditions, *condition)
}
