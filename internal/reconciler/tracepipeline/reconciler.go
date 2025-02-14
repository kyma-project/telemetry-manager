/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tracepipeline

import (
	"context"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/trace/gateway"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
	telemetryutils "github.com/kyma-project/telemetry-manager/internal/utils/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

const defaultReplicaCount int32 = 2

type GatewayConfigBuilder interface {
	Build(ctx context.Context, pipelines []telemetryv1alpha1.TracePipeline, opts gateway.BuildOptions) (*gateway.Config, otlpexporter.EnvVars, error)
}

type GatewayApplierDeleter interface {
	ApplyResources(ctx context.Context, c client.Client, opts otelcollector.GatewayApplyOptions) error
	DeleteResources(ctx context.Context, c client.Client, isIstioActive bool) error
}

type PipelineLock interface {
	TryAcquireLock(ctx context.Context, owner metav1.Object) error
	IsLockHolder(ctx context.Context, owner metav1.Object) error
}

type FlowHealthProber interface {
	Probe(ctx context.Context, pipelineName string) (prober.OTelPipelineProbeResult, error)
}

type OverridesHandler interface {
	LoadOverrides(ctx context.Context) (*overrides.Config, error)
}

type IstioStatusChecker interface {
	IsIstioActive(ctx context.Context) bool
}

type Reconciler struct {
	client.Client

	telemetryNamespace string

	// Dependencies
	flowHealthProber      FlowHealthProber
	gatewayApplierDeleter GatewayApplierDeleter
	gatewayConfigBuilder  GatewayConfigBuilder
	gatewayProber         commonstatus.Prober
	istioStatusChecker    IstioStatusChecker
	overridesHandler      OverridesHandler
	pipelineLock          PipelineLock
	pipelineValidator     *Validator
	errToMsgConverter     commonstatus.ErrorToMessageConverter
}

func New(
	client client.Client,
	telemetryNamespace string,
	flowHealthProber FlowHealthProber,
	gatewayApplierDeleter GatewayApplierDeleter,
	gatewayConfigBuilder GatewayConfigBuilder,
	gatewayProber commonstatus.Prober,
	istioStatusChecker IstioStatusChecker,
	overridesHandler OverridesHandler,
	pipelineLock PipelineLock,
	pipelineValidator *Validator,
	errToMsgConverter commonstatus.ErrorToMessageConverter,
) *Reconciler {
	return &Reconciler{
		Client:                client,
		telemetryNamespace:    telemetryNamespace,
		flowHealthProber:      flowHealthProber,
		gatewayApplierDeleter: gatewayApplierDeleter,
		gatewayConfigBuilder:  gatewayConfigBuilder,
		gatewayProber:         gatewayProber,
		istioStatusChecker:    istioStatusChecker,
		overridesHandler:      overridesHandler,
		pipelineLock:          pipelineLock,
		pipelineValidator:     pipelineValidator,
		errToMsgConverter:     errToMsgConverter,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logf.FromContext(ctx).V(1).Info("Reconciling")

	overrideConfig, err := r.overridesHandler.LoadOverrides(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideConfig.Tracing.Paused {
		logf.FromContext(ctx).V(1).Info("Skipping reconciliation: paused using override config")
		return ctrl.Result{}, nil
	}

	var tracePipeline telemetryv1alpha1.TracePipeline
	if err := r.Get(ctx, req.NamespacedName, &tracePipeline); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	err = r.doReconcile(ctx, &tracePipeline)
	if statusErr := r.updateStatus(ctx, tracePipeline.Name); statusErr != nil {
		if err != nil {
			err = fmt.Errorf("failed while updating status: %w: %w", statusErr, err)
		} else {
			err = fmt.Errorf("failed to update status: %w", statusErr)
		}
	}

	return ctrl.Result{}, err
}

func (r *Reconciler) doReconcile(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline) error {
	if err := r.pipelineLock.TryAcquireLock(ctx, pipeline); err != nil {
		return err
	}

	var allPipelinesList telemetryv1alpha1.TracePipelineList
	if err := r.List(ctx, &allPipelinesList); err != nil {
		return fmt.Errorf("failed to list trace pipelines: %w", err)
	}

	reconcilablePipelines, err := r.getReconcilablePipelines(ctx, allPipelinesList.Items)
	if err != nil {
		return fmt.Errorf("failed to fetch deployable trace pipelines: %w", err)
	}

	if len(reconcilablePipelines) == 0 {
		logf.FromContext(ctx).V(1).Info("cleaning up trace pipeline resources: all trace pipelines are non-reconcilable")

		if err = r.gatewayApplierDeleter.DeleteResources(ctx, r.Client, r.istioStatusChecker.IsIstioActive(ctx)); err != nil {
			return fmt.Errorf("failed to delete gateway resources: %w", err)
		}

		return nil
	}

	if err = r.reconcileTraceGateway(ctx, pipeline, reconcilablePipelines); err != nil {
		return fmt.Errorf("failed to reconcile trace gateway: %w", err)
	}

	return nil
}

// getReconcilablePipelines returns the list of trace pipelines that are ready to be rendered into the otel collector configuration. A pipeline is deployable if it is not being deleted, all secret references exist, and is not above the pipeline limit.
func (r *Reconciler) getReconcilablePipelines(ctx context.Context, allPipelines []telemetryv1alpha1.TracePipeline) ([]telemetryv1alpha1.TracePipeline, error) {
	var reconcilablePipelines []telemetryv1alpha1.TracePipeline

	for i := range allPipelines {
		isReconcilable, err := r.isReconcilable(ctx, &allPipelines[i])
		if err != nil {
			return nil, err
		}

		if isReconcilable {
			reconcilablePipelines = append(reconcilablePipelines, allPipelines[i])
		}
	}

	return reconcilablePipelines, nil
}

func (r *Reconciler) isReconcilable(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline) (bool, error) {
	if !pipeline.GetDeletionTimestamp().IsZero() {
		return false, nil
	}

	err := r.pipelineValidator.validate(ctx, pipeline)

	// Pipeline with a certificate that is about to expire is still considered reconcilable
	if err == nil || tlscert.IsCertAboutToExpireError(err) {
		return true, nil
	}

	// Remaining errors imply that the pipeline is not reconcilable
	// In case that one of the requests to the Kubernetes API server failed, then the pipeline is also considered non-reconcilable and the error is returned to trigger a requeue
	var APIRequestFailed *errortypes.APIRequestFailedError
	if errors.As(err, &APIRequestFailed) {
		return false, APIRequestFailed.Err
	}

	return false, nil
}

func (r *Reconciler) reconcileTraceGateway(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline, allPipelines []telemetryv1alpha1.TracePipeline) error {
	clusterInfo := k8sutils.GetGardenerShootInfo(ctx, r.Client)

	collectorConfig, collectorEnvVars, err := r.gatewayConfigBuilder.Build(ctx, allPipelines, gateway.BuildOptions{
		ClusterName:   clusterInfo.ClusterName,
		CloudProvider: clusterInfo.CloudProvider,
	})
	if err != nil {
		return fmt.Errorf("failed to create collector config: %w", err)
	}

	collectorConfigYAML, err := yaml.Marshal(collectorConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal collector config: %w", err)
	}

	isIstioActive := r.istioStatusChecker.IsIstioActive(ctx)

	allowedPorts := []int32{
		ports.OTLPHTTP,
		ports.OTLPGRPC,
		ports.Metrics,
		ports.HealthCheck,
	}

	if isIstioActive {
		allowedPorts = append(allowedPorts, ports.IstioEnvoy)
	}

	opts := otelcollector.GatewayApplyOptions{
		AllowedPorts:                   allowedPorts,
		CollectorConfigYAML:            string(collectorConfigYAML),
		CollectorEnvVars:               collectorEnvVars,
		IstioEnabled:                   isIstioActive,
		IstioExcludePorts:              []int32{ports.Metrics},
		Replicas:                       r.getReplicaCountFromTelemetry(ctx),
		ResourceRequirementsMultiplier: len(allPipelines),
	}

	if err := r.gatewayApplierDeleter.ApplyResources(
		ctx,
		k8sutils.NewOwnerReferenceSetter(r.Client, pipeline),
		opts,
	); err != nil {
		return fmt.Errorf("failed to apply gateway resources: %w", err)
	}

	return nil
}

func (r *Reconciler) getReplicaCountFromTelemetry(ctx context.Context) int32 {
	telemetry, err := telemetryutils.GetDefaultTelemetryInstance(ctx, r.Client, r.telemetryNamespace)
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to get telemetry: using default scaling")
		return defaultReplicaCount
	}

	if telemetry.Spec.Trace != nil &&
		telemetry.Spec.Trace.Gateway.Scaling.Type == operatorv1alpha1.StaticScalingStrategyType &&
		telemetry.Spec.Trace.Gateway.Scaling.Static != nil &&
		telemetry.Spec.Trace.Gateway.Scaling.Static.Replicas > 0 {
		return telemetry.Spec.Trace.Gateway.Scaling.Static.Replicas
	}

	return defaultReplicaCount
}
