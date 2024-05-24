package metricpipeline

import (
	"context"
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
	configmetricagent "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/agent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/gateway"
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

//go:generate mockery --name DaemonSetProber --filename daemonset_prober.go
type DaemonSetProber interface {
	IsReady(ctx context.Context, name types.NamespacedName) (bool, error)
}

//go:generate mockery --name FlowHealthProber --filename flow_health_prober.go
type FlowHealthProber interface {
	Probe(ctx context.Context, pipelineName string) (prober.OTelPipelineProbeResult, error)
}

//go:generate mockery --name TLSCertValidator --filename tls_cert_validator.go
type TLSCertValidator interface {
	ValidateCertificate(ctx context.Context, cert, key *telemetryv1alpha1.ValueType) error
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
	config Config

	agentConfigBuilder       *configmetricagent.Builder
	gatewayConfigBuilder     *gateway.Builder
	gatewayApplier           *otelcollector.GatewayApplier
	agentApplier             *otelcollector.AgentApplier
	pipelineLock             PipelineLock
	gatewayProber            DeploymentProber
	agentProber              DaemonSetProber
	flowHealthProbingEnabled bool
	flowHealthProber         FlowHealthProber
	tlsCertValidator         TLSCertValidator
	overridesHandler         OverridesHandler
	istioStatusChecker       IstioStatusChecker
}

func NewReconciler(
	client client.Client,
	config Config,
	gatewayProber DeploymentProber,
	agentProber DaemonSetProber,
	flowHealthProbingEnabled bool,
	flowHealthProber FlowHealthProber,
	overridesHandler *overrides.Handler,
) *Reconciler {
	return &Reconciler{
		Client: client,
		config: config,
		gatewayConfigBuilder: &gateway.Builder{
			Reader: client,
		},
		agentConfigBuilder: &configmetricagent.Builder{
			Config: configmetricagent.BuilderConfig{
				GatewayOTLPServiceName: types.NamespacedName{
					Namespace: config.Gateway.Namespace,
					Name:      config.Gateway.OTLPServiceName,
				},
			},
		},
		gatewayApplier: &otelcollector.GatewayApplier{
			Config: config.Gateway,
		},
		agentApplier: &otelcollector.AgentApplier{
			Config: config.Agent,
		},
		pipelineLock: resourcelock.New(client, types.NamespacedName{
			Name:      "telemetry-metricpipeline-lock",
			Namespace: config.Gateway.Namespace,
		}, config.MaxPipelines),
		gatewayProber:            gatewayProber,
		agentProber:              agentProber,
		flowHealthProbingEnabled: flowHealthProbingEnabled,
		flowHealthProber:         flowHealthProber,
		tlsCertValidator:         tlscert.New(client),
		overridesHandler:         overridesHandler,
		istioStatusChecker:       istiostatus.NewChecker(client),
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
		logf.FromContext(ctx).V(1).Info("Skipping reconciliation: no metric pipeline ready for deployment")
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

	if secretref.ReferencesNonExistentSecret(ctx, r.Client, pipeline) {
		return false, nil
	}

	if tlsCertValidationRequired(pipeline) {
		cert := pipeline.Spec.Output.Otlp.TLS.Cert
		key := pipeline.Spec.Output.Otlp.TLS.Key

		if err := r.tlsCertValidator.ValidateCertificate(ctx, cert, key); err != nil {
			if !tlscert.IsCertAboutToExpireError(err) {
				return false, nil
			}
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

	if err := r.gatewayApplier.ApplyResources(
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
	agentConfig := r.agentConfigBuilder.Build(allPipelines, configmetricagent.BuildOptions{
		IstioEnabled: isIstioActive,
	})

	agentConfigYAML, err := yaml.Marshal(agentConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal collector config: %w", err)
	}

	allowedPorts := getAgentPorts()
	if isIstioActive {
		allowedPorts = append(allowedPorts, ports.IstioEnvoy)
	}

	if err := r.agentApplier.ApplyResources(
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

func tlsCertValidationRequired(pipeline *telemetryv1alpha1.MetricPipeline) bool {
	otlp := pipeline.Spec.Output.Otlp
	if otlp == nil {
		return false
	}
	if otlp.TLS == nil {
		return false
	}
	return otlp.TLS.Cert != nil || otlp.TLS.Key != nil
}
