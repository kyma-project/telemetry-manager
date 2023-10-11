package metricpipeline

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/agent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/gateway"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	otelagentresources "github.com/kyma-project/telemetry-manager/internal/resources/otelcollector/agent"
	otelcoreresources "github.com/kyma-project/telemetry-manager/internal/resources/otelcollector/core"
	otelgatewayresources "github.com/kyma-project/telemetry-manager/internal/resources/otelcollector/gateway"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
)

const defaultReplicaCount int32 = 2

type Config struct {
	Agent                  otelagentresources.Config
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
	config             Config
	prober             DeploymentProber
	overridesHandler   overrides.GlobalConfigHandler
	istioStatusChecker istioStatusChecker
}

func NewReconciler(client client.Client, config Config, prober DeploymentProber, overridesHandler overrides.GlobalConfigHandler) *Reconciler {
	return &Reconciler{
		Client:             client,
		config:             config,
		prober:             prober,
		overridesHandler:   overridesHandler,
		istioStatusChecker: istioStatusChecker{client: client},
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
	if overrideConfig.Metrics.Paused {
		logf.FromContext(ctx).V(1).Info("Skipping reconciliation: paused using override config")
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

	lock := kubernetes.NewResourceCountLock(r.Client, types.NamespacedName{
		Name:      "telemetry-metricpipeline-lock",
		Namespace: r.config.Gateway.Namespace,
	}, r.config.MaxPipelines)
	if err = lock.TryAcquireLock(ctx, pipeline); err != nil {
		lockAcquired = false
		return err
	}

	var allPipelinesList telemetryv1alpha1.MetricPipelineList
	if err = r.List(ctx, &allPipelinesList); err != nil {
		return fmt.Errorf("failed to list metric pipelines: %w", err)
	}
	deployablePipelines, err := getDeployableMetricPipelines(ctx, allPipelinesList.Items, r, lock)
	if err != nil {
		return fmt.Errorf("failed to fetch deployable metric pipelines: %w", err)
	}
	if len(deployablePipelines) == 0 {
		logf.FromContext(ctx).V(1).Info("Skipping reconciliation: no metric pipeline ready for deployment")
		return nil
	}

	if err = r.reconcileMetricGateway(ctx, pipeline, deployablePipelines); err != nil {
		return fmt.Errorf("failed to reconcile metric gateway: %w", err)
	}

	if isMetricAgentRequired(pipeline) {
		if err = r.reconcileMetricAgents(ctx, pipeline, allPipelinesList.Items); err != nil {
			return fmt.Errorf("failed to reconcile metric agents: %w", err)
		}
	}

	return nil
}

// getDeployableMetricPipelines returns the list of metric pipelines that are ready to be rendered into the otel collector configuration. A pipeline is deployable if it is not being deleted, all secret references exist, and is not above the pipeline limit.
func getDeployableMetricPipelines(ctx context.Context, allPipelines []telemetryv1alpha1.MetricPipeline, client client.Client, lock *kubernetes.ResourceCountLock) ([]telemetryv1alpha1.MetricPipeline, error) {
	var deployablePipelines []telemetryv1alpha1.MetricPipeline
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

func (r *Reconciler) reconcileMetricGateway(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline, allPipelines []telemetryv1alpha1.MetricPipeline) error {
	namespacedBaseName := types.NamespacedName{
		Name:      r.config.Gateway.BaseName,
		Namespace: r.config.Gateway.Namespace,
	}

	ownerRefSetter := kubernetes.NewOwnerReferenceSetter(r.Client, pipeline)

	serviceAccount := commonresources.MakeServiceAccount(namespacedBaseName)
	if err := kubernetes.CreateOrUpdateServiceAccount(ctx, ownerRefSetter, serviceAccount); err != nil {
		return fmt.Errorf("failed to create otel collector service account: %w", err)
	}

	clusterRole := otelgatewayresources.MakeClusterRole(namespacedBaseName)
	if err := kubernetes.CreateOrUpdateClusterRole(ctx, ownerRefSetter, clusterRole); err != nil {
		return fmt.Errorf("failed to create otel collector cluster role: %w", err)
	}

	clusterRoleBinding := commonresources.MakeClusterRoleBinding(namespacedBaseName)
	if err := kubernetes.CreateOrUpdateClusterRoleBinding(ctx, ownerRefSetter, clusterRoleBinding); err != nil {
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
	if err = kubernetes.CreateOrUpdateSecret(ctx, ownerRefSetter, secret); err != nil {
		return fmt.Errorf("failed to create otel collector env secret: %w", err)
	}

	configMap := otelcoreresources.MakeConfigMap(namespacedBaseName, string(gatewayConfigYAML))
	if err = kubernetes.CreateOrUpdateConfigMap(ctx, ownerRefSetter, configMap); err != nil {
		return fmt.Errorf("failed to create otel collector configmap: %w", err)
	}

	configHash := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, []corev1.Secret{*secret})
	scaling := otelgatewayresources.Scaling{
		Replicas:                       r.getReplicaCountFromTelemetry(ctx),
		ResourceRequirementsMultiplier: len(allPipelines),
	}
	deployment := otelgatewayresources.MakeDeployment(r.config.Gateway, configHash, scaling,
		config.EnvVarCurrentPodIP, config.EnvVarCurrentNodeName)
	if err = kubernetes.CreateOrUpdateDeployment(ctx, ownerRefSetter, deployment); err != nil {
		return fmt.Errorf("failed to create otel collector deployment: %w", err)
	}

	otlpService := otelgatewayresources.MakeOTLPService(r.config.Gateway)
	if err = kubernetes.CreateOrUpdateService(ctx, ownerRefSetter, otlpService); err != nil {
		return fmt.Errorf("failed to create otel collector otlp service: %w", err)
	}

	metricsService := otelgatewayresources.MakeMetricsService(r.config.Gateway)
	if err = kubernetes.CreateOrUpdateService(ctx, ownerRefSetter, metricsService); err != nil {
		return fmt.Errorf("failed to create otel collector metrics service: %w", err)
	}

	networkPolicyPorts := makeNetworkPolicyPorts()
	networkPolicy := otelgatewayresources.MakeNetworkPolicy(r.config.Gateway, networkPolicyPorts)
	if err = kubernetes.CreateOrUpdateNetworkPolicy(ctx, ownerRefSetter, networkPolicy); err != nil {
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

func isMetricAgentRequired(pipeline *telemetryv1alpha1.MetricPipeline) bool {
	return pipeline.Spec.Input.Application.Runtime.Enabled || pipeline.Spec.Input.Application.Prometheus.Enabled || pipeline.Spec.Input.Application.Istio.Enabled
}

func (r *Reconciler) reconcileMetricAgents(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline, allPipelines []telemetryv1alpha1.MetricPipeline) error {
	namespacedBaseName := types.NamespacedName{
		Name:      r.config.Agent.BaseName,
		Namespace: r.config.Agent.Namespace,
	}

	ownerRefSetter := kubernetes.NewOwnerReferenceSetter(r.Client, pipeline)

	serviceAccount := commonresources.MakeServiceAccount(namespacedBaseName)
	if err := kubernetes.CreateOrUpdateServiceAccount(ctx, ownerRefSetter, serviceAccount); err != nil {
		return fmt.Errorf("failed to create otel collector service account: %w", err)
	}

	clusterRole := otelagentresources.MakeClusterRole(namespacedBaseName)
	if err := kubernetes.CreateOrUpdateClusterRole(ctx, ownerRefSetter, clusterRole); err != nil {
		return fmt.Errorf("failed to create otel collector cluster role: %w", err)
	}

	clusterRoleBinding := commonresources.MakeClusterRoleBinding(namespacedBaseName)
	if err := kubernetes.CreateOrUpdateClusterRoleBinding(ctx, ownerRefSetter, clusterRoleBinding); err != nil {
		return fmt.Errorf("failed to create otel collector cluster role Binding: %w", err)
	}

	isIstioActive := r.istioStatusChecker.isIstioActive(ctx)
	agentConfig := agent.MakeConfig(types.NamespacedName{
		Namespace: r.config.Gateway.Namespace,
		Name:      r.config.Gateway.Service.OTLPServiceName,
	}, allPipelines, isIstioActive)
	var agentConfigYAML []byte
	agentConfigYAML, err := yaml.Marshal(agentConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal collector config: %w", err)
	}

	configMap := otelcoreresources.MakeConfigMap(namespacedBaseName, string(agentConfigYAML))
	if err = kubernetes.CreateOrUpdateConfigMap(ctx, ownerRefSetter, configMap); err != nil {
		return fmt.Errorf("failed to create otel collector configmap: %w", err)
	}

	configHash := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, []corev1.Secret{})
	daemonSet := otelagentresources.MakeDaemonSet(r.config.Agent, configHash, config.EnvVarCurrentPodIP, config.EnvVarCurrentNodeName, agent.IstioCertPath)
	if err := kubernetes.CreateOrUpdateDaemonSet(ctx, ownerRefSetter, daemonSet); err != nil {
		return fmt.Errorf("failed to create otel collector deployment: %w", err)
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
		scaling := telemetries.Items[i].Spec.Metric.Gateway.Scaling
		if scaling.Strategy == operatorv1alpha1.StaticScalingStrategyType && scaling.Static != nil && scaling.Static.Replicas > 0 {
			return scaling.Static.Replicas
		}
	}
	return defaultReplicaCount
}
