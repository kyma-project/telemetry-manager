package otel

import (
	"context"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/log/agent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/log/gateway"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
	telemetryutils "github.com/kyma-project/telemetry-manager/internal/utils/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

const defaultReplicaCount int32 = 2

type GatewayConfigBuilder interface {
	Build(ctx context.Context, pipelines []telemetryv1alpha1.LogPipeline, opts gateway.BuildOptions) (*gateway.Config, otlpexporter.EnvVars, error)
}

type GatewayApplierDeleter interface {
	ApplyResources(ctx context.Context, c client.Client, opts otelcollector.GatewayApplyOptions) error
	DeleteResources(ctx context.Context, c client.Client, isIstioActive bool) error
}

type GatewayFlowHealthProber interface {
	Probe(ctx context.Context, pipelineName string) (prober.OTelGatewayProbeResult, error)
}

type AgentFlowHealthProber interface {
	Probe(ctx context.Context, pipelineName string) (prober.OTelAgentProbeResult, error)
}

type IstioStatusChecker interface {
	IsIstioActive(ctx context.Context) bool
}

type AgentConfigBuilder interface {
	Build(ctx context.Context, pipelines []telemetryv1alpha1.LogPipeline, options agent.BuildOptions) (*agent.Config, otlpexporter.EnvVars, error)
}

type AgentApplierDeleter interface {
	ApplyResources(ctx context.Context, c client.Client, opts otelcollector.AgentApplyOptions) error
	DeleteResources(ctx context.Context, c client.Client) error
}

// var _ logpipeline.LogPipelineReconciler = &Reconciler{}

type PipelineValidator interface {
	Validate(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error
}

type Prober interface {
	IsReady(ctx context.Context, name types.NamespacedName) error
}

type ErrorToMessageConverter interface {
	Convert(err error) string
}

type Reconciler struct {
	client.Client

	telemetryNamespace string
	moduleVersion      string

	// Dependencies
	gatewayFlowHealthProber GatewayFlowHealthProber
	agentFlowHealthProber   AgentFlowHealthProber
	agentConfigBuilder      AgentConfigBuilder
	agentProber             Prober
	agentApplierDeleter     AgentApplierDeleter
	gatewayApplierDeleter   GatewayApplierDeleter
	gatewayConfigBuilder    GatewayConfigBuilder
	gatewayProber           Prober
	istioStatusChecker      IstioStatusChecker
	pipelineLock            PipelineLock
	pipelineValidator       PipelineValidator
	errToMessageConverter   ErrorToMessageConverter
}

func New(
	client client.Client,
	telemetryNamespace string,
	moduleVersion string,
	gatewayFlowHeathProber GatewayFlowHealthProber,
	agentFlowHealthProber AgentFlowHealthProber,
	agentConfigBuilder AgentConfigBuilder,
	agentApplierDeleter AgentApplierDeleter,
	agentProber Prober,
	gatewayApplierDeleter GatewayApplierDeleter,
	gatewayConfigBuilder GatewayConfigBuilder,
	gatewayProber Prober,
	istioStatusChecker IstioStatusChecker,
	pipelineLock PipelineLock,
	pipelineValidator PipelineValidator,
	errToMessageConverter ErrorToMessageConverter,
) *Reconciler {
	return &Reconciler{
		Client:                  client,
		telemetryNamespace:      telemetryNamespace,
		moduleVersion:           moduleVersion,
		gatewayFlowHealthProber: gatewayFlowHeathProber,
		agentFlowHealthProber:   agentFlowHealthProber,
		agentConfigBuilder:      agentConfigBuilder,
		agentApplierDeleter:     agentApplierDeleter,
		agentProber:             agentProber,
		gatewayApplierDeleter:   gatewayApplierDeleter,
		gatewayConfigBuilder:    gatewayConfigBuilder,
		gatewayProber:           gatewayProber,
		istioStatusChecker:      istioStatusChecker,
		pipelineLock:            pipelineLock,
		pipelineValidator:       pipelineValidator,
		errToMessageConverter:   errToMessageConverter,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	logf.FromContext(ctx).V(1).Info("Reconciling LogPipeline")

	err := r.doReconcile(ctx, pipeline)

	if statusErr := r.updateStatus(ctx, pipeline.Name); statusErr != nil {
		if err != nil {
			err = fmt.Errorf("failed while updating status: %w: %w", statusErr, err)
		} else {
			err = fmt.Errorf("failed to update status: %w", statusErr)
		}
	}

	return err
}

func (r *Reconciler) SupportedOutput() logpipelineutils.Mode {
	return logpipelineutils.OTel
}

func (r *Reconciler) doReconcile(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	if err := r.pipelineLock.TryAcquireLock(ctx, pipeline); err != nil {
		if errors.Is(err, resourcelock.ErrMaxPipelinesExceeded) {
			logf.FromContext(ctx).V(1).Info("Skipping reconciliation: maximum pipeline count limit exceeded")
			return nil
		}

		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	allPipelines, err := logpipelineutils.GetPipelinesForType(ctx, r.Client, r.SupportedOutput())
	if err != nil {
		return err
	}

	reconcilablePipelines, err := r.getReconcilablePipelines(ctx, allPipelines)
	if err != nil {
		return fmt.Errorf("failed to fetch deployable log pipelines: %w", err)
	}

	var reconcilablePipelinesRequiringAgents = r.getPipelinesRequiringAgents(reconcilablePipelines)

	if len(reconcilablePipelinesRequiringAgents) == 0 {
		logf.FromContext(ctx).V(1).Info("cleaning up log agent resources: no log pipelines require an agent")

		if err = r.agentApplierDeleter.DeleteResources(ctx, r.Client); err != nil {
			return fmt.Errorf("failed to delete agent resources: %w", err)
		}
	}

	if len(reconcilablePipelines) == 0 {
		logf.FromContext(ctx).V(1).Info("cleaning up log pipeline resources: all log pipelines are non-reconcilable")

		if err = r.gatewayApplierDeleter.DeleteResources(ctx, r.Client, r.istioStatusChecker.IsIstioActive(ctx)); err != nil {
			return fmt.Errorf("failed to delete gateway resources: %w", err)
		}

		return nil
	}

	if err := r.reconcileLogGateway(ctx, pipeline, reconcilablePipelines); err != nil {
		return fmt.Errorf("failed to reconcile log gateway: %w", err)
	}

	if len(reconcilablePipelinesRequiringAgents) > 0 {
		if err := r.reconcileLogAgent(ctx, pipeline, reconcilablePipelinesRequiringAgents); err != nil {
			return fmt.Errorf("failed to reconcile log agent: %w", err)
		}
	}

	return nil
}

// getReconcilablePipelines returns the list of log pipelines that are ready to be rendered into the otel collector configuration.
// A pipeline is deployable if it is not being deleted, all secret references exist, and is not above the pipeline limit.
func (r *Reconciler) getReconcilablePipelines(ctx context.Context, allPipelines []telemetryv1alpha1.LogPipeline) ([]telemetryv1alpha1.LogPipeline, error) {
	var reconcilablePipelines []telemetryv1alpha1.LogPipeline

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

func (r *Reconciler) isReconcilable(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) (bool, error) {
	if !pipeline.GetDeletionTimestamp().IsZero() {
		return false, nil
	}

	err := r.pipelineValidator.Validate(ctx, pipeline)

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

func (r *Reconciler) reconcileLogGateway(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline, allPipelines []telemetryv1alpha1.LogPipeline) error {
	clusterInfo := k8sutils.GetGardenerShootInfo(ctx, r.Client)
	collectorConfig, collectorEnvVars, err := r.gatewayConfigBuilder.Build(ctx, allPipelines, gateway.BuildOptions{
		ClusterName:   clusterInfo.ClusterName,
		CloudProvider: clusterInfo.CloudProvider,
		Enrichments:   telemetryutils.GetEnrichmentsFromTelemetry(ctx, r.Client, r.telemetryNamespace),
	})

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

func (r *Reconciler) reconcileLogAgent(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline, allPipelines []telemetryv1alpha1.LogPipeline) error {
	agentConfig, envVars, err := r.agentConfigBuilder.Build(ctx, allPipelines, agent.BuildOptions{
		InstrumentationScopeVersion: r.moduleVersion,
		AgentNamespace:              r.telemetryNamespace,
		ClusterName:                 k8sutils.GetGardenerShootInfo(ctx, r.Client).ClusterName,
		CloudProvider:               k8sutils.GetGardenerShootInfo(ctx, r.Client).CloudProvider,
		Enrichments:                 telemetryutils.GetEnrichmentsFromTelemetry(ctx, r.Client, r.telemetryNamespace),
	})
	if err != nil {
		return fmt.Errorf("failed to build agent config: %w", err)
	}

	agentConfigYAML, err := yaml.Marshal(agentConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal agent config: %w", err)
	}

	isIstioActive := r.istioStatusChecker.IsIstioActive(ctx)
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
			CollectorEnvVars:    envVars,
		},
	); err != nil {
		return fmt.Errorf("failed to apply agent resources: %w", err)
	}

	return nil
}

func (r *Reconciler) getReplicaCountFromTelemetry(ctx context.Context) int32 {
	telemetry, err := telemetryutils.GetDefaultTelemetryInstance(ctx, r.Client, r.telemetryNamespace)
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to get telemetry: using default scaling")
		return defaultReplicaCount
	}

	if telemetry.Spec.Log != nil &&
		telemetry.Spec.Log.Gateway.Scaling.Type == operatorv1alpha1.StaticScalingStrategyType &&
		telemetry.Spec.Log.Gateway.Scaling.Static != nil && telemetry.Spec.Log.Gateway.Scaling.Static.Replicas > 0 {
		return telemetry.Spec.Log.Gateway.Scaling.Static.Replicas
	}

	return defaultReplicaCount
}

func getGatewayPorts() []int32 {
	return []int32{
		ports.Metrics,
		ports.HealthCheck,
		ports.OTLPHTTP,
		ports.OTLPGRPC,
	}
}

func getAgentPorts() []int32 {
	return []int32{
		ports.Metrics,
		ports.HealthCheck,
	}
}

func (r *Reconciler) getPipelinesRequiringAgents(allPipelines []telemetryv1alpha1.LogPipeline) []telemetryv1alpha1.LogPipeline {
	var pipelinesRequiringAgents = make([]telemetryv1alpha1.LogPipeline, 0)

	for i := range allPipelines {
		if isLogAgentRequired(&allPipelines[i]) {
			pipelinesRequiringAgents = append(pipelinesRequiringAgents, allPipelines[i])
		}
	}

	return pipelinesRequiringAgents
}

func isLogAgentRequired(pipeline *telemetryv1alpha1.LogPipeline) bool {
	input := pipeline.Spec.Input

	return input.Application != nil && input.Application.Enabled != nil && *input.Application.Enabled
}
