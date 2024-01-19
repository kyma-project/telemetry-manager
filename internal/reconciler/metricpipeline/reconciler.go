package metricpipeline

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
	configmetricagent "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/agent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/gateway"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
)

const defaultReplicaCount int32 = 2

type Config struct {
	Agent                  otelcollector.AgentConfig
	Gateway                otelcollector.GatewayConfig
	OverridesConfigMapName types.NamespacedName
	MaxPipelines           int
}

//go:generate mockery --name DeploymentProber --filename deployment_prober.go
type DeploymentProber interface {
	IsReady(ctx context.Context, name types.NamespacedName) (bool, error)
}

//go:generate mockery --name DaemonSetProber --filename daemonset_prober.go
type DaemonSetProber interface {
	IsReady(ctx context.Context, name types.NamespacedName) (bool, error)
}

type Reconciler struct {
	client.Client
	config             Config
	gatewayProber      DeploymentProber
	agentProber        DaemonSetProber
	overridesHandler   *overrides.Handler
	istioStatusChecker istiostatus.Checker
}

func NewReconciler(client client.Client, config Config, gatewayProber DeploymentProber, agentProber DaemonSetProber, overridesHandler *overrides.Handler) *Reconciler {
	return &Reconciler{
		Client:             client,
		config:             config,
		gatewayProber:      gatewayProber,
		agentProber:        agentProber,
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

	lock := k8sutils.NewResourceCountLock(r.Client, types.NamespacedName{
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
func getDeployableMetricPipelines(ctx context.Context, allPipelines []telemetryv1alpha1.MetricPipeline, client client.Client, lock *k8sutils.ResourceCountLock) ([]telemetryv1alpha1.MetricPipeline, error) {
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

func isMetricAgentRequired(pipeline *telemetryv1alpha1.MetricPipeline) bool {
	input := pipeline.Spec.Input
	isRuntimeInputEnabled := input.Runtime != nil && input.Runtime.Enabled
	isPrometheusInputEnabled := input.Prometheus != nil && input.Prometheus.Enabled
	isIstioInputEnabled := input.Istio != nil && input.Istio.Enabled
	return isRuntimeInputEnabled || isPrometheusInputEnabled || isIstioInputEnabled
}

func (r *Reconciler) reconcileMetricGateway(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline, allPipelines []telemetryv1alpha1.MetricPipeline) error {
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

	allowedPorts := getGatewayPorts()
	if isIstioActive {
		allowedPorts = append(allowedPorts, ports.IstioEnvoy)
	}

	if err := otelcollector.ApplyGatewayResources(ctx,
		k8sutils.NewOwnerReferenceSetter(r.Client, pipeline),
		r.config.Gateway.WithScaling(scaling).WithCollectorConfig(string(collectorConfigYAML), collectorEnvVars).
			WithIstioConfig(fmt.Sprintf("%d", ports.Metrics), isIstioActive).
			WithAllowedPorts(allowedPorts)); err != nil {
		return fmt.Errorf("failed to apply gateway resources: %w", err)
	}

	return nil
}

func (r *Reconciler) reconcileMetricAgents(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline, allPipelines []telemetryv1alpha1.MetricPipeline) error {
	isIstioActive := r.istioStatusChecker.IsIstioActive(ctx)
	agentConfig := configmetricagent.MakeConfig(types.NamespacedName{
		Namespace: r.config.Gateway.Namespace,
		Name:      r.config.Gateway.OTLPServiceName,
	}, allPipelines, isIstioActive)

	agentConfigYAML, err := yaml.Marshal(agentConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal collector config: %w", err)
	}

	allowedPorts := getAgentPorts()
	if isIstioActive {
		allowedPorts = append(allowedPorts, ports.IstioEnvoy)

	}

	if err := otelcollector.ApplyAgentResources(ctx,
		k8sutils.NewOwnerReferenceSetter(r.Client, pipeline),
		r.config.Agent.WithCollectorConfig(string(agentConfigYAML)).
			WithAllowedPorts(allowedPorts)); err != nil {
		return fmt.Errorf("failed to apply agent resources: %w", err)
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
		if telemetrySpec.Metric == nil {
			continue
		}

		scaling := telemetrySpec.Metric.Gateway.Scaling
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

func getAgentPorts() []int32 {
	return []int32{
		ports.Metrics,
		ports.HealthCheck,
	}
}

func getGatewayPorts() []int32 {
	return []int32{
		ports.Metrics,
		ports.HealthCheck,
		ports.OTLPHTTP,
		ports.OTLPGRPC,
	}
}
