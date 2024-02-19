package tracepipeline

import (
	"context"
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Reconciler) updateStatus(ctx context.Context, pipelineName string, withinPipelineCountLimit bool) error {
	var pipeline telemetryv1alpha1.TracePipeline
	if err := r.Get(ctx, types.NamespacedName{Name: pipelineName}, &pipeline); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to get TracePipeline: %v", err)
	}

	if pipeline.DeletionTimestamp != nil {
		return nil
	}

	if !withinPipelineCountLimit {
		conditions.SetPendingCondition(ctx, &pipeline.Status.Conditions, pipeline.Generation, conditions.ReasonMaxPipelinesExceeded, pipeline.Name)
		return updateStatus(ctx, r.Client, &pipeline)
	}

	referencesNonExistentSecret := secretref.ReferencesNonExistentSecret(ctx, r.Client, &pipeline)
	if referencesNonExistentSecret {
		conditions.SetPendingCondition(ctx, &pipeline.Status.Conditions, pipeline.Generation, conditions.ReasonReferencedSecretMissing, pipeline.Name)
		return updateStatus(ctx, r.Client, &pipeline)
	}

	gatewayReady, err := r.prober.IsReady(ctx, types.NamespacedName{Name: r.config.Gateway.BaseName, Namespace: r.config.Gateway.Namespace})
	if err != nil {
		return err
	}

	if !gatewayReady {
		conditions.SetPendingCondition(ctx, &pipeline.Status.Conditions, pipeline.Generation, conditions.ReasonTraceGatewayDeploymentNotReady, pipeline.Name)
		return updateStatus(ctx, r.Client, &pipeline)
	}

	conditions.SetRunningCondition(ctx, &pipeline.Status.Conditions, pipeline.Generation, conditions.ReasonTraceGatewayDeploymentReady, pipeline.Name)
	return updateStatus(ctx, r.Client, &pipeline)
}

func updateStatus(ctx context.Context, client client.Client, pipeline *telemetryv1alpha1.TracePipeline) error {
	if err := client.Status().Update(ctx, pipeline); err != nil {
		return fmt.Errorf("failed to update TracePipeline status: %w", err)
	}
	return nil
}
