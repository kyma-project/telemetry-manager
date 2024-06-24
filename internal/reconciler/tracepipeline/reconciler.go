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
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/istiostatus"
	"github.com/kyma-project/telemetry-manager/internal/k8sutils"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/trace/gateway"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/tlscert"
)

const defaultReplicaCount int32 = 2

type Config struct {
	Gateway                otelcollector.GatewayConfig
	OverridesConfigMapName types.NamespacedName
	MaxPipelines           int
}

//go:generate mockery --name GatewayConfigBuilder --filename gateway_config_builder.go
type GatewayConfigBuilder interface {
	Build(ctx context.Context, pipelines []telemetryv1alpha1.TracePipeline) (*gateway.Config, otlpexporter.EnvVars, error)
}

type GatewayResourcesHandler interface {
	ApplyResources(ctx context.Context, c client.Client, opts otelcollector.GatewayApplyOptions) error
	DeleteResources(ctx context.Context, c client.Client, isIstioActive bool) error
}

//go:generate mockery --name PipelineLock --filename pipeline_lock.go
type PipelineLock interface {
	TryAcquireLock(ctx context.Context, owner metav1.Object) error
	IsLockHolder(ctx context.Context, owner metav1.Object) (bool, error)
}

//go:generate mockery --name DeploymentProber --filename deployment_prober.go
type DeploymentProber interface {
	IsReady(ctx context.Context, name types.NamespacedName) (bool, error)
}

//go:generate mockery --name FlowHealthProber --filename flow_health_prober.go
type FlowHealthProber interface {
	Probe(ctx context.Context, pipelineName string) (prober.OTelPipelineProbeResult, error)
}

type TLSCertValidator interface {
	Validate(ctx context.Context, config tlscert.TLSBundle) error
}

//go:generate mockery --name OverridesHandler --filename overrides_handler.go
type OverridesHandler interface {
	LoadOverrides(ctx context.Context) (*overrides.Config, error)
}

//go:generate mockery --name IstioStatusChecker --filename istio_status_checker.go
type IstioStatusChecker interface {
	IsIstioActive(ctx context.Context) bool
}

type Reconciler struct {
	client.Client
	config                     Config
	pipelinesConditionsCleared bool

	gatewayConfigBuilder    GatewayConfigBuilder
	gatewayResourcesHandler GatewayResourcesHandler
	pipelineLock            PipelineLock
	prober                  DeploymentProber
	flowHealthProber        FlowHealthProber
	tlsCertValidator        TLSCertValidator
	overridesHandler        OverridesHandler
	istioStatusChecker      IstioStatusChecker
}

func NewReconciler(client client.Client,
	config Config,
	prober DeploymentProber,
	flowHealthProber FlowHealthProber,
	overridesHandler *overrides.Handler) *Reconciler {
	return &Reconciler{
		Client: client,
		config: config,
		gatewayConfigBuilder: &gateway.Builder{
			Reader: client,
		},
		gatewayResourcesHandler: &otelcollector.GatewayResourcesHandler{
			Config: config.Gateway,
		},
		pipelineLock: resourcelock.New(client,
			types.NamespacedName{
				Name:      "telemetry-tracepipeline-lock",
				Namespace: config.Gateway.Namespace,
			}, config.MaxPipelines),
		prober:             prober,
		flowHealthProber:   flowHealthProber,
		tlsCertValidator:   tlscert.New(client),
		overridesHandler:   overridesHandler,
		istioStatusChecker: istiostatus.NewChecker(client),
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

	return ctrl.Result{}, r.doReconcile(ctx, &tracePipeline)
}

func (r *Reconciler) doReconcile(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline) error {
	var err error
	lockAcquired := true

	defer func() {
		if statusErr := r.updateStatus(ctx, pipeline.Name, lockAcquired); statusErr != nil {
			if err != nil {
				err = fmt.Errorf("failed while updating status: %w: %w", statusErr, err)
			} else {
				err = fmt.Errorf("failed to update status: %w", statusErr)
			}
		}
	}()

	if err = r.pipelineLock.TryAcquireLock(ctx, pipeline); err != nil {
		lockAcquired = false
		return err
	}

	var allPipelinesList telemetryv1alpha1.TracePipelineList
	if err = r.List(ctx, &allPipelinesList); err != nil {
		return fmt.Errorf("failed to list trace pipelines: %w", err)
	}

	if err = r.clearPipelinesConditions(ctx, allPipelinesList.Items); err != nil {
		return fmt.Errorf("failed to clear the conditions list for trace pipelines: %w", err)
	}

	reconcilablePipelines, err := r.getReconcilablePipelines(ctx, allPipelinesList.Items)
	if err != nil {
		return fmt.Errorf("failed to fetch deployable trace pipelines: %w", err)
	}

	if len(reconcilablePipelines) == 0 {
		logf.FromContext(ctx).V(1).Info("cleaning up trace pipeline resources: all trace pipelines are non-reconcilable")
		if err = r.gatewayResourcesHandler.DeleteResources(ctx, r.Client, r.istioStatusChecker.IsIstioActive(ctx)); err != nil {
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

	if err := secretref.VerifySecretReference(ctx, r.Client, pipeline); err != nil {
		if errors.Is(err, secretref.ErrSecretRefNotFound) || errors.Is(err, secretref.ErrSecretKeyNotFound) {
			return false, nil
		}
		return false, err
	}

	if tlsValidationRequired(pipeline) {
		tlsConfig := tlscert.TLSBundle{
			Cert: pipeline.Spec.Output.Otlp.TLS.Cert,
			Key:  pipeline.Spec.Output.Otlp.TLS.Key,
			CA:   pipeline.Spec.Output.Otlp.TLS.CA,
		}

		if err := r.tlsCertValidator.Validate(ctx, tlsConfig); err != nil {
			return tlscert.IsCertAboutToExpireError(err), nil
		}
	}
	hasLock, err := r.pipelineLock.IsLockHolder(ctx, pipeline)
	if err != nil {
		return false, fmt.Errorf("failed to check lock: %w", err)
	}
	return hasLock, nil

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

	if err := r.gatewayResourcesHandler.ApplyResources(
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

func tlsValidationRequired(pipeline *telemetryv1alpha1.TracePipeline) bool {
	otlp := pipeline.Spec.Output.Otlp
	if otlp == nil {
		return false
	}
	if otlp.TLS == nil {
		return false
	}

	return otlp.TLS.Cert != nil || otlp.TLS.Key != nil || otlp.TLS.CA != nil
}

// clearPipelinesConditions clears the status conditions for all TracePipelines only in the 1st reconciliation
// This is done to allow the legacy conditions ("Running" and "Pending") to be always appended at the end of the conditions list even if new condition types are added
// Check https://github.com/kyma-project/telemetry-manager/blob/main/docs/contributor/arch/004-consolidate-pipeline-statuses.md#decision
// TODO: Remove this logic after the end of the deprecation period of the legacy conditions ("Running" and "Pending")
func (r *Reconciler) clearPipelinesConditions(ctx context.Context, allPipelines []telemetryv1alpha1.TracePipeline) error {
	if r.pipelinesConditionsCleared {
		return nil
	}

	for i := range allPipelines {
		allPipelines[i].Status.Conditions = []metav1.Condition{}
		if err := r.Status().Update(ctx, &allPipelines[i]); err != nil {
			return fmt.Errorf("failed to update TracePipeline status: %w", err)
		}
	}
	r.pipelinesConditionsCleared = true

	return nil
}
