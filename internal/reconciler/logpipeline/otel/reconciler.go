package otel

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/labels"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/log/gateway"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
)

const defaultReplicaCount int32 = 2

type Config struct {
	LogGatewayName     string
	TelemetryNamespace string
}

type GatewayConfigBuilder interface {
	Build(ctx context.Context, pipelines []telemetryv1alpha1.LogPipeline) (*gateway.Config, otlpexporter.EnvVars, error)
}

type GatewayApplierDeleter interface {
	ApplyResources(ctx context.Context, c client.Client, opts otelcollector.GatewayApplyOptions) error
	DeleteResources(ctx context.Context, c client.Client, isIstioActive bool) error
}

type FlowHealthProber interface {
	Probe(ctx context.Context, pipelineName string) (prober.OTelPipelineProbeResult, error)
}

var _ logpipeline.LogPipelineReconciler = &Reconciler{}

type Reconciler struct {
	client.Client

	config Config

	// Dependencies
	flowHealthProber      FlowHealthProber
	gatewayApplierDeleter GatewayApplierDeleter
	gatewayConfigBuilder  GatewayConfigBuilder
	gatewayProber         commonstatus.DeploymentProber
	pipelineValidator     *Validator
	errToMessageConverter commonstatus.ErrorToMessageConverter
}

func New(
	client client.Client,
	config Config,
	gatewayApplierDeleter GatewayApplierDeleter,
	gatewayConfigBuilder GatewayConfigBuilder,
	gatewayProber commonstatus.DeploymentProber,
	pipelineValidator *Validator,
	errToMessageConverter commonstatus.ErrorToMessageConverter,
) *Reconciler {
	return &Reconciler{
		Client:                client,
		config:                config,
		gatewayApplierDeleter: gatewayApplierDeleter,
		gatewayConfigBuilder:  gatewayConfigBuilder,
		gatewayProber:         gatewayProber,
		pipelineValidator:     pipelineValidator,
		errToMessageConverter: errToMessageConverter,
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
	allPipelines, err := logpipeline.GetPipelinesForType(ctx, r.Client, r.SupportedOutput())
	if err != nil {
		return err
	}

	// TODO: Check if no pipeline is reconcilable. Not a priority, do this later.

	if err := r.reconcileLogGateway(ctx, pipeline, allPipelines); err != nil {
		return fmt.Errorf("failed to reconcile log gateway: %w", err)
	}

	return nil
}

func (r *Reconciler) reconcileLogGateway(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline, allPipelines []telemetryv1alpha1.LogPipeline) error {
	collectorConfig, collectorEnvVars, err := r.gatewayConfigBuilder.Build(ctx, allPipelines)
	if err != nil {
		return fmt.Errorf("failed to create collector config: %w", err)
	}

	collectorConfigYAML, err := yaml.Marshal(collectorConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal collector config: %w", err)
	}

	logGatewaySelectorLabels := labels.MakeLogGatewaySelectorLabel(r.config.LogGatewayName)

	opts := otelcollector.GatewayApplyOptions{
		CollectorConfigYAML:            string(collectorConfigYAML),
		CollectorEnvVars:               collectorEnvVars,
		ComponentSelectorLabels:        logGatewaySelectorLabels,
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
