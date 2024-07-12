package metricpipeline

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
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/agent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/gateway"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
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
	Agent                  otelcollector.AgentConfig
	Gateway                otelcollector.GatewayConfig
	OverridesConfigMapName types.NamespacedName
	MaxPipelines           int
	ModuleVersion          string
}

type AgentConfigBuilder interface {
	Build(pipelines []telemetryv1alpha1.MetricPipeline, options agent.BuildOptions) *agent.Config
}

type GatewayConfigBuilder interface {
	Build(ctx context.Context, pipelines []telemetryv1alpha1.MetricPipeline) (*gateway.Config, otlpexporter.EnvVars, error)
}

type AgentApplierDeleter interface {
	ApplyResources(ctx context.Context, c client.Client, opts otelcollector.AgentApplyOptions) error
	DeleteResources(ctx context.Context, c client.Client) error
}

type GatewayApplierDeleter interface {
	ApplyResources(ctx context.Context, c client.Client, opts otelcollector.GatewayApplyOptions) error
	DeleteResources(ctx context.Context, c client.Client, isIstioActive bool) error
}

type PipelineLock interface {
	TryAcquireLock(ctx context.Context, owner metav1.Object) error
	IsLockHolder(ctx context.Context, owner metav1.Object) (bool, error)
}

type DeploymentProber interface {
	IsReady(ctx context.Context, name types.NamespacedName) (bool, error)
}

type DaemonSetProber interface {
	IsReady(ctx context.Context, name types.NamespacedName) (bool, error)
}

type FlowHealthProber interface {
	Probe(ctx context.Context, pipelineName string) (prober.OTelPipelineProbeResult, error)
}

type TLSCertValidator interface {
	Validate(ctx context.Context, config tlscert.TLSBundle) error
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

	agentConfigBuilder    AgentConfigBuilder
	gatewayConfigBuilder  GatewayConfigBuilder
	agentApplierDeleter   AgentApplierDeleter
	gatewayApplierDeleter GatewayApplierDeleter
	pipelineLock          PipelineLock
	gatewayProber         DeploymentProber
	agentProber           DaemonSetProber
	flowHealthProber      FlowHealthProber
	tlsCertValidator      TLSCertValidator
	overridesHandler      OverridesHandler
	istioStatusChecker    IstioStatusChecker
}

func New(
	client client.Client,
	config Config,
	gatewayProber DeploymentProber,
	agentProber DaemonSetProber,
	flowHealthProber FlowHealthProber,
	overridesHandler *overrides.Handler,
) *Reconciler {
	return &Reconciler{
		Client: client,
		config: config,
		gatewayConfigBuilder: &gateway.Builder{
			Reader: client,
		},
		agentConfigBuilder: &agent.Builder{
			Config: agent.BuilderConfig{
				GatewayOTLPServiceName: types.NamespacedName{
					Namespace: config.Gateway.Namespace,
					Name:      config.Gateway.OTLPServiceName,
				},
			},
		},
		gatewayApplierDeleter: &otelcollector.GatewayApplierDeleter{
			Config: config.Gateway,
		},
		agentApplierDeleter: &otelcollector.AgentApplierDeleter{
			Config: config.Agent,
		},
		pipelineLock: resourcelock.New(client, types.NamespacedName{
			Name:      "telemetry-metricpipeline-lock",
			Namespace: config.Gateway.Namespace,
		}, config.MaxPipelines),
		gatewayProber:      gatewayProber,
		agentProber:        agentProber,
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

	var allPipelinesList telemetryv1alpha1.MetricPipelineList
	if err = r.List(ctx, &allPipelinesList); err != nil {
		return fmt.Errorf("failed to list metric pipelines: %w", err)
	}

	reconcilablePipelines, err := r.getReconcilablePipelines(ctx, allPipelinesList.Items)
	if err != nil {
		return fmt.Errorf("failed to fetch deployable metric pipelines: %w", err)
	}

	if len(reconcilablePipelines) == 0 {
		logf.FromContext(ctx).V(1).Info("cleaning up metric pipeline resources: all metric pipelines are non-reconcilable")
		if err = r.gatewayApplierDeleter.DeleteResources(ctx, r.Client, r.istioStatusChecker.IsIstioActive(ctx)); err != nil {
			return fmt.Errorf("failed to delete gateway resources: %w", err)
		}
		if err = r.agentApplierDeleter.DeleteResources(ctx, r.Client); err != nil {
			return fmt.Errorf("failed to delete agent resources: %w", err)
		}
		return nil
	}

	if err = r.reconcileMetricGateway(ctx, pipeline, reconcilablePipelines); err != nil {
		return fmt.Errorf("failed to reconcile metric gateway: %w", err)
	}

	if isMetricAgentRequired(pipeline) {
		if err = r.reconcileMetricAgents(ctx, pipeline, allPipelinesList.Items); err != nil {
			return fmt.Errorf("failed to reconcile metric agents: %w", err)
		}
	}

	return nil
}

// getReconcilablePipelines returns the list of metric pipelines that are ready to be rendered into the otel collector configuration. A pipeline is deployable if it is not being deleted, all secret references exist, and is not above the pipeline limit.
func (r *Reconciler) getReconcilablePipelines(ctx context.Context, allPipelines []telemetryv1alpha1.MetricPipeline) ([]telemetryv1alpha1.MetricPipeline, error) {
	var reconcilablePipelines []telemetryv1alpha1.MetricPipeline
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

func (r *Reconciler) isReconcilable(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline) (bool, error) {
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

func isMetricAgentRequired(pipeline *telemetryv1alpha1.MetricPipeline) bool {
	input := pipeline.Spec.Input
	isRuntimeInputEnabled := input.Runtime != nil && input.Runtime.Enabled
	isPrometheusInputEnabled := input.Prometheus != nil && input.Prometheus.Enabled
	isIstioInputEnabled := input.Istio != nil && input.Istio.Enabled
	return isRuntimeInputEnabled || isPrometheusInputEnabled || isIstioInputEnabled
}

func (r *Reconciler) reconcileMetricGateway(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline, allPipelines []telemetryv1alpha1.MetricPipeline) error {
	collectorConfig, collectorEnvVars, err := r.gatewayConfigBuilder.Build(ctx, allPipelines)
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

func (r *Reconciler) reconcileMetricAgents(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline, allPipelines []telemetryv1alpha1.MetricPipeline) error {
	isIstioActive := r.istioStatusChecker.IsIstioActive(ctx)
	agentConfig := r.agentConfigBuilder.Build(allPipelines, agent.BuildOptions{
		IstioEnabled:                isIstioActive,
		IstioCertPath:               otelcollector.IstioCertPath,
		InstrumentationScopeVersion: r.config.ModuleVersion,
	})

	agentConfigYAML, err := yaml.Marshal(agentConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal collector config: %w", err)
	}

	allowedPorts := getAgentPorts()
	if isIstioActive {
		allowedPorts = append(allowedPorts, ports.IstioEnvoy)
	}

	if err := r.agentApplierDeleter.ApplyResources(
		ctx,
		k8sutils.NewOwnerReferenceSetter(r.Client, pipeline),
		otelcollector.AgentApplyOptions{
			AllowedPorts:        allowedPorts,
			CollectorConfigYAML: string(agentConfigYAML),
		},
	); err != nil {
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

func tlsValidationRequired(pipeline *telemetryv1alpha1.MetricPipeline) bool {
	otlp := pipeline.Spec.Output.Otlp
	if otlp == nil {
		return false
	}
	if otlp.TLS == nil {
		return false
	}
	return otlp.TLS.Cert != nil || otlp.TLS.Key != nil || otlp.TLS.CA != nil
}
