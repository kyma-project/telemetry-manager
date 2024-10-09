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
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/k8sutils"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/trace/gateway"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

const defaultReplicaCount int32 = 2

type Config struct {
	Gateway      otelcollector.GatewayConfig
	MaxPipelines int
}

type GatewayConfigBuilder interface {
	Build(ctx context.Context, pipelines []telemetryv1alpha1.TracePipeline) (*gateway.Config, otlpexporter.EnvVars, error)
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

	config Config

	// Dependencies
	flowHealthProber      FlowHealthProber
	gatewayApplierDeleter GatewayApplierDeleter
	gatewayConfigBuilder  GatewayConfigBuilder
	gatewayProber         commonstatus.DeploymentProber
	istioStatusChecker    IstioStatusChecker
	overridesHandler      OverridesHandler
	pipelineLock          PipelineLock
	pipelineValidator     *Validator
	errToMsgConverter     commonstatus.ErrorToMessageConverter
}

func New(
	client client.Client,
	config Config,
	flowHealthProber FlowHealthProber,
	gatewayApplierDeleter GatewayApplierDeleter,
	gatewayConfigBuilder GatewayConfigBuilder,
	gatewayProber commonstatus.DeploymentProber,
	istioStatusChecker IstioStatusChecker,
	overridesHandler OverridesHandler,
	pipelineLock PipelineLock,
	pipelineValidator *Validator,
	errToMsgConverter commonstatus.ErrorToMessageConverter,
) *Reconciler {
	return &Reconciler{
		Client:                client,
		config:                config,
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

	// TODO: Delete this logic after next release on regular (+ increase coverage threshold back to 74%)
	if err := r.cleanUpOldTraceCollectorResources(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to clean up old trace collector resources: %w", err)
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
	collectorConfig, collectorEnvVars, err := r.gatewayConfigBuilder.Build(ctx, allPipelines)
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
	var telemetries operatorv1alpha1.TelemetryList
	if err := r.List(ctx, &telemetries); err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to list telemetry: using default scaling")
		return defaultReplicaCount
	}
	for i := range telemetries.Items {
		telemetrySpec := telemetries.Items[i].Spec
		if telemetrySpec.Trace == nil {
			continue
		}

		scaling := telemetrySpec.Trace.Gateway.Scaling
		if scaling.Type != operatorv1alpha1.StaticScalingStrategyType {
			continue
		}

		static := scaling.Static
		if static != nil && static.Replicas > 0 {
			return static.Replicas
		}
	}
	return defaultReplicaCount
}

func (r *Reconciler) cleanUpOldTraceCollectorResources(ctx context.Context) error {
	oldTraceCollectorResources := []client.Object{
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "telemetry-trace-collector",
				Namespace: "kyma-system",
			},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "telemetry-trace-collector",
			},
		},
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "telemetry-trace-collector",
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "telemetry-trace-collector-metrics",
				Namespace: "kyma-system",
			},
		},
		&networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "telemetry-trace-collector",
				Namespace: "kyma-system",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "telemetry-trace-collector",
				Namespace: "kyma-system",
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "telemetry-trace-collector",
				Namespace: "kyma-system",
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "telemetry-trace-collector",
				Namespace: "kyma-system",
			},
		},
	}

	if r.istioStatusChecker.IsIstioActive(ctx) {
		oldTraceCollectorResources = append(oldTraceCollectorResources,
			&istiosecurityclientv1.PeerAuthentication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "telemetry-trace-collector",
					Namespace: "kyma-system",
				},
			})
	}

	var errs error
	for _, oldResource := range oldTraceCollectorResources {
		if err := r.Delete(ctx, oldResource); err != nil && !apierrors.IsNotFound(err) {
			errs = errors.Join(errs, fmt.Errorf("failed to delete old trace collector resource: %w", err))
		}
	}

	return errs
}
