package metricpipeline

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metricagent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metricgateway"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
	metricpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/metricpipeline"
	telemetryutils "github.com/kyma-project/telemetry-manager/internal/utils/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

const defaultReplicaCount int32 = 2

type AgentConfigBuilder interface {
	Build(ctx context.Context, pipelines []telemetryv1alpha1.MetricPipeline, options metricagent.BuildOptions) (*common.Config, error)
}

type GatewayConfigBuilder interface {
	Build(ctx context.Context, pipelines []telemetryv1alpha1.MetricPipeline, options metricgateway.BuildOptions) (*common.Config, common.EnvVars, error)
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
	IsLockHolder(ctx context.Context, owner metav1.Object) error
}

type PipelineSyncer interface {
	TryAcquireLock(ctx context.Context, owner metav1.Object) error
}

type FlowHealthProber interface {
	Probe(ctx context.Context, pipelineName string) (prober.OTelGatewayProbeResult, error)
}

type OverridesHandler interface {
	LoadOverrides(ctx context.Context) (*overrides.Config, error)
}

type IstioStatusChecker interface {
	IsIstioActive(ctx context.Context) bool
}

type Reconciler struct {
	client.Client

	telemetryNamespace string
	// TODO(skhalash): introduce an embed pkg exposing the module version set by go build
	moduleVersion string

	agentApplierDeleter   AgentApplierDeleter
	agentConfigBuilder    AgentConfigBuilder
	agentProber           commonstatus.Prober
	flowHealthProber      FlowHealthProber
	gatewayApplierDeleter GatewayApplierDeleter
	gatewayConfigBuilder  GatewayConfigBuilder
	gatewayProber         commonstatus.Prober
	istioStatusChecker    IstioStatusChecker
	overridesHandler      OverridesHandler
	pipelineLock          PipelineLock
	pipelineSync          PipelineSyncer
	pipelineValidator     *Validator
	errToMsgConverter     commonstatus.ErrorToMessageConverter
}

func New(
	client client.Client,
	telemetryNamespace string,
	moduleVersion string,
	agentApplierDeleter AgentApplierDeleter,
	agentConfigBuilder AgentConfigBuilder,
	agentProber commonstatus.Prober,
	flowHealthProber FlowHealthProber,
	gatewayApplierDeleter GatewayApplierDeleter,
	gatewayConfigBuilder GatewayConfigBuilder,
	gatewayProber commonstatus.Prober,
	istioStatusChecker IstioStatusChecker,
	overridesHandler OverridesHandler,
	pipelineLock PipelineLock,
	pipelineSync PipelineSyncer,
	pipelineValidator *Validator,
	errToMsgConverter commonstatus.ErrorToMessageConverter,
) *Reconciler {
	return &Reconciler{
		Client:                client,
		telemetryNamespace:    telemetryNamespace,
		moduleVersion:         moduleVersion,
		agentApplierDeleter:   agentApplierDeleter,
		agentConfigBuilder:    agentConfigBuilder,
		agentProber:           agentProber,
		flowHealthProber:      flowHealthProber,
		gatewayApplierDeleter: gatewayApplierDeleter,
		gatewayConfigBuilder:  gatewayConfigBuilder,
		gatewayProber:         gatewayProber,
		istioStatusChecker:    istioStatusChecker,
		overridesHandler:      overridesHandler,
		pipelineLock:          pipelineLock,
		pipelineSync:          pipelineSync,
		pipelineValidator:     pipelineValidator,
		errToMsgConverter:     errToMsgConverter,
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

	if err := r.pipelineSync.TryAcquireLock(ctx, &metricPipeline); err != nil {
		if errors.Is(err, resourcelock.ErrMaxPipelinesExceeded) {
			logf.FromContext(ctx).V(1).Error(err, "Could not register pipeline")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	err = r.doReconcile(ctx, &metricPipeline)
	if statusErr := r.updateStatus(ctx, metricPipeline.Name); statusErr != nil {
		if err != nil {
			err = fmt.Errorf("failed while updating status: %w: %w", statusErr, err)
		} else {
			err = fmt.Errorf("failed to update status: %w", statusErr)
		}
	}

	return ctrl.Result{}, err
}

func (r *Reconciler) doReconcile(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline) error {
	if err := r.pipelineLock.TryAcquireLock(ctx, pipeline); err != nil {
		if errors.Is(err, resourcelock.ErrMaxPipelinesExceeded) {
			logf.FromContext(ctx).V(1).Info("Skipping reconciliation: maximum pipeline count limit exceeded")
			return nil
		}

		return err
	}

	var allPipelinesList telemetryv1alpha1.MetricPipelineList
	if err := r.List(ctx, &allPipelinesList); err != nil {
		return fmt.Errorf("failed to list metric pipelines: %w", err)
	}

	reconcilablePipelines, err := r.getReconcilablePipelines(ctx, allPipelinesList.Items)
	if err != nil {
		return fmt.Errorf("failed to fetch deployable metric pipelines: %w", err)
	}

	var reconcilablePipelinesRequiringAgents = r.getPipelinesRequiringAgents(reconcilablePipelines)

	if len(reconcilablePipelinesRequiringAgents) == 0 {
		logf.FromContext(ctx).V(1).Info("cleaning up metric agent resources: no metric pipelines require an agent")

		if err = r.agentApplierDeleter.DeleteResources(ctx, r.Client); err != nil {
			return fmt.Errorf("failed to delete agent resources: %w", err)
		}
	}

	if len(reconcilablePipelines) == 0 {
		logf.FromContext(ctx).V(1).Info("cleaning up metric pipeline resources: all metric pipelines are non-reconcilable")

		if err = r.gatewayApplierDeleter.DeleteResources(ctx, r.Client, r.istioStatusChecker.IsIstioActive(ctx)); err != nil {
			return fmt.Errorf("failed to delete gateway resources: %w", err)
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

	err := r.pipelineValidator.validate(ctx, pipeline)

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

func (r *Reconciler) getPipelinesRequiringAgents(allPipelines []telemetryv1alpha1.MetricPipeline) []telemetryv1alpha1.MetricPipeline {
	var pipelinesRequiringAgents []telemetryv1alpha1.MetricPipeline

	for i := range allPipelines {
		if isMetricAgentRequired(&allPipelines[i]) {
			pipelinesRequiringAgents = append(pipelinesRequiringAgents, allPipelines[i])
		}
	}

	return pipelinesRequiringAgents
}

func isMetricAgentRequired(pipeline *telemetryv1alpha1.MetricPipeline) bool {
	input := pipeline.Spec.Input

	return metricpipelineutils.IsRuntimeInputEnabled(input) || metricpipelineutils.IsPrometheusInputEnabled(input) || metricpipelineutils.IsIstioInputEnabled(input)
}

func (r *Reconciler) reconcileMetricGateway(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline, allPipelines []telemetryv1alpha1.MetricPipeline) error {
	shootInfo := k8sutils.GetGardenerShootInfo(ctx, r.Client)
	clusterName := r.getClusterNameFromTelemetry(ctx, shootInfo.ClusterName)

	clusterUID, err := r.getK8sClusterUID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get kube-system namespace for cluster UID: %w", err)
	}

	var enrichments *operatorv1alpha1.EnrichmentSpec

	t, err := telemetryutils.GetDefaultTelemetryInstance(ctx, r.Client, r.telemetryNamespace)
	if err == nil {
		enrichments = t.Spec.Enrichments
	}

	collectorConfig, collectorEnvVars, err := r.gatewayConfigBuilder.Build(ctx, allPipelines, metricgateway.BuildOptions{
		GatewayNamespace:            r.telemetryNamespace,
		InstrumentationScopeVersion: r.moduleVersion,
		ClusterName:                 clusterName,
		ClusterUID:                  clusterUID,
		CloudProvider:               shootInfo.CloudProvider,
		Enrichments:                 enrichments,
	})
	if err != nil {
		return fmt.Errorf("failed to create collector config: %w", err)
	}

	collectorConfigYAML, err := yaml.Marshal(collectorConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal collector config: %w", err)
	}

	isIstioActive := r.istioStatusChecker.IsIstioActive(ctx)

	opts := otelcollector.GatewayApplyOptions{
		CollectorConfigYAML:            string(collectorConfigYAML),
		CollectorEnvVars:               collectorEnvVars,
		IstioEnabled:                   isIstioActive,
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

	agentConfig, err := r.agentConfigBuilder.Build(ctx, allPipelines, metricagent.BuildOptions{
		IstioEnabled:                isIstioActive,
		IstioCertPath:               otelcollector.IstioCertPath,
		InstrumentationScopeVersion: r.moduleVersion,
		AgentNamespace:              r.telemetryNamespace,
	})
	if err != nil {
		return fmt.Errorf("failed to create collector config: %w", err)
	}

	agentConfigYAML, err := yaml.Marshal(agentConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal collector config: %w", err)
	}

	backendPorts, err := r.getBackendPorts(ctx, allPipelines)
	if err != nil {
		return fmt.Errorf("failed to get ports of the backends: %w", err)
	}

	if err := r.agentApplierDeleter.ApplyResources(
		ctx,
		k8sutils.NewOwnerReferenceSetter(r.Client, pipeline),
		otelcollector.AgentApplyOptions{
			IstioEnabled:        isIstioActive,
			CollectorConfigYAML: string(agentConfigYAML),
			BackendPorts:        backendPorts,
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

	if telemetry.Spec.Metric != nil &&
		telemetry.Spec.Metric.Gateway.Scaling.Type == operatorv1alpha1.StaticScalingStrategyType &&
		telemetry.Spec.Metric.Gateway.Scaling.Static != nil &&
		telemetry.Spec.Metric.Gateway.Scaling.Static.Replicas > 0 {
		return telemetry.Spec.Metric.Gateway.Scaling.Static.Replicas
	}

	return defaultReplicaCount
}

func (r *Reconciler) getClusterNameFromTelemetry(ctx context.Context, defaultName string) string {
	telemetry, err := telemetryutils.GetDefaultTelemetryInstance(ctx, r.Client, r.telemetryNamespace)
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to get telemetry: using default shoot name as cluster name")
		return defaultName
	}

	if telemetry.Spec.Enrichments != nil &&
		telemetry.Spec.Enrichments.Cluster != nil &&
		telemetry.Spec.Enrichments.Cluster.Name != "" {
		return telemetry.Spec.Enrichments.Cluster.Name
	}

	return defaultName
}

func (r *Reconciler) getK8sClusterUID(ctx context.Context) (string, error) {
	var kubeSystem corev1.Namespace

	kubeSystemNs := types.NamespacedName{
		Name: "kube-system",
	}

	err := r.Get(ctx, kubeSystemNs, &kubeSystem)
	if err != nil {
		return "", err
	}

	return string(kubeSystem.UID), nil
}

// getBackendPorts returns the list of ports of the backends defined in all given MetricPipelines
func (r *Reconciler) getBackendPorts(ctx context.Context, allPipelines []telemetryv1alpha1.MetricPipeline) ([]string, error) {
	var backendPorts []string

	for _, pipeline := range allPipelines {
		endpoint, err := common.ResolveValue(ctx, r.Client, pipeline.Spec.Output.OTLP.Endpoint)
		if err != nil {
			return nil, err
		}

		parsedURL, err := url.Parse(string(endpoint))
		if err != nil {
			return nil, err
		}

		backendPorts = append(backendPorts, parsedURL.Port())
	}

	// List of ports needs to be sorted
	// Otherwise, metric agent will continuously restart, because in each reconciliation we can have the ports list in a different order
	slices.Sort(backendPorts)
	// Remove duplication in ports in case multiple backends are defined with the same port
	backendPorts = slices.Compact(backendPorts)

	return backendPorts, nil
}
