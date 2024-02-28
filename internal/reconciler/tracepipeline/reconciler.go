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
	"fmt"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/istiostatus"
	"github.com/kyma-project/telemetry-manager/internal/k8sutils"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/trace/gateway"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
)

const defaultReplicaCount int32 = 2

type Config struct {
	Gateway                otelcollector.GatewayConfig
	OverridesConfigMapName types.NamespacedName
	MaxPipelines           int
}

//go:generate mockery --name DeploymentProber --filename deployment_prober.go
type DeploymentProber interface {
	IsReady(ctx context.Context, name types.NamespacedName) (bool, error)
}

type Reconciler struct {
	client.Client
	config             Config
	prober             DeploymentProber
	overridesHandler   *overrides.Handler
	istioStatusChecker istiostatus.Checker
}

func NewReconciler(client client.Client, config Config, prober DeploymentProber, overridesHandler *overrides.Handler) *Reconciler {
	return &Reconciler{
		Client:             client,
		config:             config,
		prober:             prober,
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
				err = fmt.Errorf("failed while updating status: %v: %v", statusErr, err)
			} else {
				err = fmt.Errorf("failed to update status: %v", statusErr)
			}
		}
	}()

	lock := k8sutils.NewResourceCountLock(r.Client, types.NamespacedName{
		Name:      "telemetry-tracepipeline-lock",
		Namespace: r.config.Gateway.Namespace,
	}, r.config.MaxPipelines)
	if err = lock.TryAcquireLock(ctx, pipeline); err != nil {
		lockAcquired = false
		return err
	}

	var allPipelinesList telemetryv1alpha1.TracePipelineList
	if err = r.List(ctx, &allPipelinesList); err != nil {
		return fmt.Errorf("failed to list trace pipelines: %w", err)
	}
	deployablePipelines, err := getDeployableTracePipelines(ctx, allPipelinesList.Items, r, lock)
	if err != nil {
		return fmt.Errorf("failed to fetch deployable trace pipelines: %w", err)
	}
	if len(deployablePipelines) == 0 {
		logf.FromContext(ctx).V(1).Info("Skipping reconciliation: no trace pipeline ready for deployment")
		return nil
	}

	if err = r.reconcileTraceGateway(ctx, pipeline, deployablePipelines); err != nil {
		return fmt.Errorf("failed to reconcile trace gateway: %w", err)
	}

	return nil
}

// getDeployableTracePipelines returns the list of trace pipelines that are ready to be rendered into the otel collector configuration. A pipeline is deployable if it is not being deleted, all secret references exist, and is not above the pipeline limit.
func getDeployableTracePipelines(ctx context.Context, allPipelines []telemetryv1alpha1.TracePipeline, client client.Client, lock *k8sutils.ResourceCountLock) ([]telemetryv1alpha1.TracePipeline, error) {
	var deployablePipelines []telemetryv1alpha1.TracePipeline
	for i := range allPipelines {
		if !allPipelines[i].GetDeletionTimestamp().IsZero() {
			continue
		}

		if secretref.ReferencesNonExistentSecret(ctx, client, &allPipelines[i]) {
			continue
		}

		hasLock, err := lock.IsLockHolder(ctx, &allPipelines[i])
		if err != nil {
			return nil, err
		}

		if hasLock {
			deployablePipelines = append(deployablePipelines, allPipelines[i])
		}
	}
	return deployablePipelines, nil
}

func (r *Reconciler) reconcileTraceGateway(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline, allPipelines []telemetryv1alpha1.TracePipeline) error {
	scaling := otelcollector.GatewayScalingConfig{
		Replicas:                       r.getReplicaCountFromTelemetry(ctx),
		ResourceRequirementsMultiplier: len(allPipelines),
	}

	collectorConfig, collectorEnvVars, err := gateway.MakeConfig(ctx, r.Client, allPipelines)
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
		allowedPorts = append(allowedPorts, ports.IstioEnvoy, ports.OpenCensus)
	}

	if err := otelcollector.ApplyGatewayResources(ctx,
		k8sutils.NewOwnerReferenceSetter(r.Client, pipeline),
		r.config.Gateway.WithScaling(scaling).WithCollectorConfig(string(collectorConfigYAML), collectorEnvVars).
			WithIstioConfig(fmt.Sprintf("%d, %d", ports.Metrics, ports.OpenCensus), isIstioActive).
			WithAllowedPorts(allowedPorts)); err != nil {
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
