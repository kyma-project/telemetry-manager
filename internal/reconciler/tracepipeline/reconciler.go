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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/trace/gateway"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	otelcoreresources "github.com/kyma-project/telemetry-manager/internal/resources/otelcollector/core"
	otelgatewayresources "github.com/kyma-project/telemetry-manager/internal/resources/otelcollector/gateway"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
)

type Config struct {
	Gateway                otelgatewayresources.Config
	OverridesConfigMapName types.NamespacedName
	MaxPipelines           int
}

//go:generate mockery --name DeploymentProber --filename deployment_prober.go
type DeploymentProber interface {
	IsReady(ctx context.Context, name types.NamespacedName) (bool, error)
}

type Reconciler struct {
	client.Client
	config           Config
	prober           DeploymentProber
	overridesHandler overrides.GlobalConfigHandler
}

func NewReconciler(client client.Client, config Config, prober DeploymentProber, overridesHandler overrides.GlobalConfigHandler) *Reconciler {
	return &Reconciler{
		Client:           client,
		config:           config,
		prober:           prober,
		overridesHandler: overridesHandler,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logf.FromContext(ctx).V(1).Info("Reconciliation triggered")

	overrideConfig, err := r.overridesHandler.UpdateOverrideConfig(ctx, r.config.OverridesConfigMapName)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.overridesHandler.CheckGlobalConfig(overrideConfig.Global); err != nil {
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

	lock := kubernetes.NewResourceCountLock(r.Client, types.NamespacedName{
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
func getDeployableTracePipelines(ctx context.Context, allPipelines []telemetryv1alpha1.TracePipeline, client client.Client, lock *kubernetes.ResourceCountLock) ([]telemetryv1alpha1.TracePipeline, error) {
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
	namespacedBaseName := types.NamespacedName{
		Name:      r.config.Gateway.BaseName,
		Namespace: r.config.Gateway.Namespace,
	}

	var err error
	serviceAccount := commonresources.MakeServiceAccount(namespacedBaseName)
	if err = controllerutil.SetOwnerReference(pipeline, serviceAccount, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateServiceAccount(ctx, r, serviceAccount); err != nil {
		return fmt.Errorf("failed to create otel collector service account: %w", err)
	}

	clusterRole := otelgatewayresources.MakeClusterRole(namespacedBaseName)
	if err = controllerutil.SetOwnerReference(pipeline, clusterRole, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateClusterRole(ctx, r, clusterRole); err != nil {
		return fmt.Errorf("failed to create otel collector cluster role: %w", err)
	}

	clusterRoleBinding := commonresources.MakeClusterRoleBinding(namespacedBaseName)
	if err = controllerutil.SetOwnerReference(pipeline, clusterRoleBinding, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateClusterRoleBinding(ctx, r, clusterRoleBinding); err != nil {
		return fmt.Errorf("failed to create otel collector cluster role Binding: %w", err)
	}

	gatewayConfig, envVars, err := gateway.MakeConfig(ctx, r, allPipelines)
	if err != nil {
		return fmt.Errorf("failed to make otel collector config: %v", err)
	}

	var gatewayConfigYAML []byte
	gatewayConfigYAML, err = yaml.Marshal(gatewayConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal collector config: %w", err)
	}

	secret := otelgatewayresources.MakeSecret(r.config.Gateway, envVars)
	if err = controllerutil.SetOwnerReference(pipeline, secret, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateSecret(ctx, r.Client, secret); err != nil {
		return fmt.Errorf("failed to create otel collector env secret: %w", err)
	}

	configMap := otelcoreresources.MakeConfigMap(namespacedBaseName, string(gatewayConfigYAML))
	if err = controllerutil.SetOwnerReference(pipeline, configMap, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateConfigMap(ctx, r.Client, configMap); err != nil {
		return fmt.Errorf("failed to create otel collector configmap: %w", err)
	}

	configHash := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, []corev1.Secret{*secret})
	deployment := otelgatewayresources.MakeDeployment(r.config.Gateway, configHash, len(allPipelines),
		config.EnvVarCurrentPodIP, config.EnvVarCurrentNodeName)
	if err = controllerutil.SetOwnerReference(pipeline, deployment, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateDeployment(ctx, r.Client, deployment); err != nil {
		return fmt.Errorf("failed to create otel collector deployment: %w", err)
	}

	otlpService := otelgatewayresources.MakeOTLPService(r.config.Gateway)
	if err = controllerutil.SetOwnerReference(pipeline, otlpService, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateService(ctx, r.Client, otlpService); err != nil {
		return fmt.Errorf("failed to create otel collector otlp service: %w", err)
	}

	openCensusService := otelgatewayresources.MakeOpenCensusService(r.config.Gateway)
	if err = controllerutil.SetOwnerReference(pipeline, openCensusService, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateService(ctx, r.Client, openCensusService); err != nil {
		return fmt.Errorf("failed to create otel collector open census service: %w", err)
	}

	metricsService := otelgatewayresources.MakeMetricsService(r.config.Gateway)
	if err = controllerutil.SetOwnerReference(pipeline, metricsService, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateService(ctx, r.Client, metricsService); err != nil {
		return fmt.Errorf("failed to create otel collector metrics service: %w", err)
	}

	networkPolicyPorts := makeNetworkPolicyPorts()
	networkPolicy := otelgatewayresources.MakeNetworkPolicy(r.config.Gateway, networkPolicyPorts)
	if err = controllerutil.SetOwnerReference(pipeline, networkPolicy, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateNetworkPolicy(ctx, r.Client, networkPolicy); err != nil {
		return fmt.Errorf("failed to create otel collector network policy: %w", err)
	}

	return nil
}

func makeNetworkPolicyPorts() []intstr.IntOrString {
	return []intstr.IntOrString{
		intstr.FromInt(ports.OTLPHTTP),
		intstr.FromInt(ports.OTLPGRPC),
		intstr.FromInt(ports.OpenCensus),
		intstr.FromInt(ports.Metrics),
		intstr.FromInt(ports.HealthCheck),
	}
}
