package metricpipeline

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
)

func (r *Reconciler) updateStatus(ctx context.Context, pipelineName string, lockAcquired bool) error {
	log := logf.FromContext(ctx)

	var pipeline telemetryv1alpha1.MetricPipeline
	if err := r.Get(ctx, types.NamespacedName{Name: pipelineName}, &pipeline); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to get MetricPipeline: %v", err)
	}

	if pipeline.DeletionTimestamp != nil {
		return nil
	}

	if !lockAcquired {
		pending := telemetryv1alpha1.NewMetricPipelineCondition(conditions.ReasonWaitingForLock, telemetryv1alpha1.MetricPipelinePending)

		if pipeline.Status.HasCondition(telemetryv1alpha1.MetricPipelineRunning) {
			log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s. Resetting previous conditions", pipeline.Name, pending.Type))
			pipeline.Status.Conditions = []telemetryv1alpha1.MetricPipelineCondition{}
		}

		return setCondition(ctx, r.Client, &pipeline, pending)
	}

	referencesNonExistentSecret := secretref.ReferencesNonExistentSecret(ctx, r.Client, &pipeline)
	if referencesNonExistentSecret {
		pending := telemetryv1alpha1.NewMetricPipelineCondition(conditions.ReasonReferencedSecretMissing, telemetryv1alpha1.MetricPipelinePending)

		if pipeline.Status.HasCondition(telemetryv1alpha1.MetricPipelineRunning) {
			log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s. Resetting previous conditions", pipeline.Name, pending.Type))
			pipeline.Status.Conditions = []telemetryv1alpha1.MetricPipelineCondition{}
		}

		return setCondition(ctx, r.Client, &pipeline, pending)
	}

	return r.updateGatewayAndAgentStatus(ctx, pipeline)

}

func (r *Reconciler) updateGatewayAndAgentStatus(ctx context.Context, pipeline telemetryv1alpha1.MetricPipeline) error {
	log := logf.FromContext(ctx)

	gatewayStatus, err := r.determineGatewayStatus(ctx)

	if err != nil {
		return err
	}

	agentEnabled := isMetricAgentRequired(&pipeline)

	if gatewayStatus.Type == telemetryv1alpha1.MetricPipelineRunning && agentEnabled {
		agentStatus, err := r.determineAgentStatus(ctx)

		if err != nil {
			return err
		}

		if pipeline.Status.HasCondition(telemetryv1alpha1.MetricPipelineRunning) && agentStatus.Type == telemetryv1alpha1.MetricPipelinePending {
			log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s. Resetting previous conditions", pipeline.Name, telemetryv1alpha1.MetricPipelinePending))
			pipeline.Status.Conditions = []telemetryv1alpha1.MetricPipelineCondition{}
		}
		return setCondition(ctx, r.Client, &pipeline, agentStatus)
	}

	if pipeline.Status.HasCondition(telemetryv1alpha1.MetricPipelineRunning) && gatewayStatus.Type == telemetryv1alpha1.MetricPipelinePending {
		log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s. Resetting previous conditions", pipeline.Name, telemetryv1alpha1.MetricPipelinePending))
		pipeline.Status.Conditions = []telemetryv1alpha1.MetricPipelineCondition{}
	}
	return setCondition(ctx, r.Client, &pipeline, gatewayStatus)
}

func (r *Reconciler) determineGatewayStatus(ctx context.Context) (*telemetryv1alpha1.MetricPipelineCondition, error) {
	gatewayReady, err := r.prober.IsReady(ctx, types.NamespacedName{Name: r.config.Gateway.BaseName, Namespace: r.config.Gateway.Namespace})
	if err != nil {
		return nil, err
	}

	if gatewayReady {
		return telemetryv1alpha1.NewMetricPipelineCondition(conditions.ReasonMetricGatewayDeploymentReady, telemetryv1alpha1.MetricPipelineRunning), nil
	}

	return telemetryv1alpha1.NewMetricPipelineCondition(conditions.ReasonMetricGatewayDeploymentNotReady, telemetryv1alpha1.MetricPipelinePending), nil
}

func (r *Reconciler) determineAgentStatus(ctx context.Context) (*telemetryv1alpha1.MetricPipelineCondition, error) {
	agentReady, err := r.agentProber.IsReady(ctx, types.NamespacedName{Name: r.config.Agent.BaseName, Namespace: r.config.Agent.Namespace})
	if err != nil {
		return nil, err
	}

	if agentReady {
		return telemetryv1alpha1.NewMetricPipelineCondition(conditions.ReasonMetricAgentDaemonSetReady, telemetryv1alpha1.MetricPipelineRunning), nil
	}

	return telemetryv1alpha1.NewMetricPipelineCondition(conditions.ReasonMetricAgentDaemonSetNotReady, telemetryv1alpha1.MetricPipelinePending), nil
}

func setCondition(ctx context.Context, client client.Client, pipeline *telemetryv1alpha1.MetricPipeline, condition *telemetryv1alpha1.MetricPipelineCondition) error {
	log := logf.FromContext(ctx)

	log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s", pipeline.Name, condition.Type))

	pipeline.Status.SetCondition(*condition)

	if err := client.Status().Update(ctx, pipeline); err != nil {
		return fmt.Errorf("failed to update MetricPipeline status to %s: %v", condition.Type, err)
	}
	return nil
}
