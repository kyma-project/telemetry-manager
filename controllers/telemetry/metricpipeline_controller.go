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

package telemetry

import (
	"context"
	"fmt"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/collector"
	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	collectorresources "github.com/kyma-project/telemetry-manager/internal/resources/collector"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
)

//go:generate mockery --name DeploymentProber --filename deployment_prober.go
type DeploymentProber interface {
	IsReady(ctx context.Context, name types.NamespacedName) (bool, error)
}

// MetricPipelineReconciler reconciles a MetricPipeline object
type MetricPipelineReconciler struct {
	client.Client
	config           collectorresources.Config
	prober           DeploymentProber
	overridesHandler overrides.GlobalConfigHandler
}

func NewReconciler(client client.Client, config collectorresources.Config, prober DeploymentProber, handler *overrides.Handler) *MetricPipelineReconciler {
	var r MetricPipelineReconciler
	r.Client = client
	r.config = config
	r.prober = prober
	r.overridesHandler = handler
	return &r
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MetricPipeline object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *MetricPipelineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	log.V(1).Info("Reconciliation triggered")

	overrideConfig, err := r.overridesHandler.UpdateOverrideConfig(ctx, r.config.OverrideConfigMap)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.overridesHandler.CheckGlobalConfig(overrideConfig.Global); err != nil {
		return ctrl.Result{}, err
	}
	if overrideConfig.Metrics.Paused {
		log.V(1).Info("Skipping reconciliation of metricpipeline as reconciliation is paused")
		return ctrl.Result{}, nil
	}

	var metricPipeline telemetryv1alpha1.MetricPipeline

	if err := r.Get(ctx, req.NamespacedName, &metricPipeline); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return ctrl.Result{}, r.doReconcile(ctx, &metricPipeline)
}

func (r *MetricPipelineReconciler) doReconcile(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline) error {
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

	lockName := types.NamespacedName{
		Name:      "telemetry-metricpipeline-lock",
		Namespace: r.config.Namespace,
	}
	if err = kubernetes.TryAcquireLock(ctx, r, lockName, pipeline); err != nil {
		lockAcquired = false
		return err
	}

	namespacedBaseName := types.NamespacedName{
		Name:      r.config.BaseName,
		Namespace: r.config.Namespace,
	}
	serviceAccount := commonresources.MakeServiceAccount(namespacedBaseName)
	if err = controllerutil.SetControllerReference(pipeline, serviceAccount, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateServiceAccount(ctx, r, serviceAccount); err != nil {
		return fmt.Errorf("failed to create otel collector service account: %w", err)
	}
	clusterRole := commonresources.MakeClusterRole(namespacedBaseName)
	if err = controllerutil.SetControllerReference(pipeline, clusterRole, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateClusterRole(ctx, r, clusterRole); err != nil {
		return fmt.Errorf("failed to create otel collector cluster role: %w", err)
	}
	clusterRoleBinding := commonresources.MakeClusterRoleBinding(namespacedBaseName)
	if err = controllerutil.SetControllerReference(pipeline, clusterRoleBinding, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateClusterRoleBinding(ctx, r, clusterRoleBinding); err != nil {
		return fmt.Errorf("failed to create otel collector cluster role Binding: %w", err)
	}

	var secretData map[string][]byte
	if secretData, err = kubernetes.FetchSecretData(ctx, r, pipeline.Spec.Output.Otlp); err != nil {
		return err
	}
	secret := collectorresources.MakeSecret(r.config, secretData)
	if err = controllerutil.SetControllerReference(pipeline, secret, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateSecret(ctx, r.Client, secret); err != nil {
		return err
	}
	endpoint := string(secretData[collector.EndpointVariable])
	isInsecure := isInsecureOutput(endpoint)

	collectorConfig := makeOtelCollectorConfig(pipeline.Spec.Output, isInsecure)
	configMap := collectorresources.MakeConfigMap(r.config, collectorConfig)
	if err = controllerutil.SetControllerReference(pipeline, configMap, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateConfigMap(ctx, r.Client, configMap); err != nil {
		return fmt.Errorf("failed to create otel collector configmap: %w", err)
	}

	configHash := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, []corev1.Secret{*secret})
	deployment := collectorresources.MakeDeployment(r.config, configHash)
	if err = controllerutil.SetControllerReference(pipeline, deployment, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateDeployment(ctx, r.Client, deployment); err != nil {
		return fmt.Errorf("failed to create otel collector deployment: %w", err)
	}

	otlpService := collectorresources.MakeOTLPService(r.config)
	if err = controllerutil.SetControllerReference(pipeline, otlpService, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateService(ctx, r.Client, otlpService); err != nil {
		return fmt.Errorf("failed to create otel collector collector service: %w", err)
	}

	metricsService := collectorresources.MakeMetricsService(r.config)
	if err = controllerutil.SetControllerReference(pipeline, metricsService, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateService(ctx, r.Client, metricsService); err != nil {
		return fmt.Errorf("failed to create otel collector metrics service: %w", err)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MetricPipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&telemetryv1alpha1.MetricPipeline{}).
		Complete(r)
}

func isInsecureOutput(endpoint string) bool {
	return len(strings.TrimSpace(endpoint)) > 0 && strings.HasPrefix(endpoint, "http://")
}
