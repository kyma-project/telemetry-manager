package metricpipeline

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	collectorresources "github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

//go:generate mockery --name DeploymentProber --filename deployment_prober.go
type DeploymentProber interface {
	IsReady(ctx context.Context, name types.NamespacedName) (bool, error)
}

type Reconciler struct {
	client.Client
	config           collectorresources.Config
	prober           DeploymentProber
	overridesHandler overrides.GlobalConfigHandler
}

func NewReconciler(client client.Client, config collectorresources.Config, prober DeploymentProber, overridesHandler overrides.GlobalConfigHandler) *Reconciler {
	return &Reconciler{
		Client:           client,
		config:           config,
		prober:           prober,
		overridesHandler: overridesHandler,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

func (r *Reconciler) doReconcile(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline) error {
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
	lock := kubernetes.NewResourceCountLock(r.Client, lockName, r.config.MaxPipelines)
	if err = lock.TryAcquireLock(ctx, pipeline); err != nil {
		lockAcquired = false
		return err
	}

	namespacedBaseName := types.NamespacedName{
		Name:      r.config.BaseName,
		Namespace: r.config.Namespace,
	}
	serviceAccount := commonresources.MakeServiceAccount(namespacedBaseName)
	if err = controllerutil.SetOwnerReference(pipeline, serviceAccount, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateServiceAccount(ctx, r, serviceAccount); err != nil {
		return fmt.Errorf("failed to create otel collector service account: %w", err)
	}
	clusterRole := commonresources.MakeClusterRole(namespacedBaseName)
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

	var metricPipelineList telemetryv1alpha1.MetricPipelineList
	if err = r.List(ctx, &metricPipelineList); err != nil {
		return fmt.Errorf("failed to list metric pipelines: %w", err)
	}
	collectorConfig, envVars, err := makeOtelCollectorConfig(ctx, r, metricPipelineList.Items)
	if err != nil {
		return fmt.Errorf("failed to make otel collector config: %v", err)
	}

	secret := collectorresources.MakeSecret(r.config, envVars)
	if err = controllerutil.SetOwnerReference(pipeline, secret, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateSecret(ctx, r.Client, secret); err != nil {
		return err
	}

	configMap := collectorresources.MakeConfigMap(r.config, *collectorConfig)
	if err = controllerutil.SetOwnerReference(pipeline, configMap, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateConfigMap(ctx, r.Client, configMap); err != nil {
		return fmt.Errorf("failed to create otel collector configmap: %w", err)
	}

	configHash := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, []corev1.Secret{*secret})
	deployment := collectorresources.MakeDeployment(r.config, configHash)
	if err = controllerutil.SetOwnerReference(pipeline, deployment, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateDeployment(ctx, r.Client, deployment); err != nil {
		return fmt.Errorf("failed to create otel collector deployment: %w", err)
	}

	otlpService := collectorresources.MakeOTLPService(r.config)
	if err = controllerutil.SetOwnerReference(pipeline, otlpService, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateService(ctx, r.Client, otlpService); err != nil {
		//nolint:dupword // otel collector collector service is a real name.
		return fmt.Errorf("failed to create otel collector collector service: %w", err)
	}

	metricsService := collectorresources.MakeMetricsService(r.config)
	if err = controllerutil.SetOwnerReference(pipeline, metricsService, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateService(ctx, r.Client, metricsService); err != nil {
		return fmt.Errorf("failed to create otel collector metrics service: %w", err)
	}

	networkPolicyPorts := makeNetworkPolicyPorts()
	networkPolicy := collectorresources.MakeNetworkPolicy(r.config, networkPolicyPorts)
	if err = controllerutil.SetOwnerReference(pipeline, networkPolicy, r.Scheme()); err != nil {
		return err
	}
	if err = kubernetes.CreateOrUpdateNetworkPolicy(ctx, r.Client, networkPolicy); err != nil {
		return fmt.Errorf("failed to create otel collector network policy: %w", err)
	}

	return nil
}
