package metricgateway

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
)

type buildComponentFunc = common.BuildComponentFunc[*telemetryv1alpha1.MetricPipeline]

type Builder struct {
	common.ComponentBuilder[*telemetryv1alpha1.MetricPipeline]

	Reader client.Reader
}

type BuildOptions struct {
	GatewayNamespace            string
	InstrumentationScopeVersion string
	ClusterName                 string
	ClusterUID                  string
	CloudProvider               string
	Enrichments                 *operatorv1alpha1.EnrichmentSpec
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.MetricPipeline, opts BuildOptions) (*common.Config, common.EnvVars, error) {
	b.Config = common.NewConfig()
	b.AddExtension(common.ComponentIDK8sLeaderElectorExtension,
		common.K8sLeaderElector{
			AuthType:       "serviceAccount",
			LeaseName:      common.K8sLeaderElectorKymaStats,
			LeaseNamespace: opts.GatewayNamespace,
		},
	)
	b.EnvVars = make(common.EnvVars)

	queueSize := common.MetricsBatchingMaxQueueSize / len(pipelines)

	for _, pipeline := range pipelines {
		inputPipelineID := formatInputMetricServicePipelineID(&pipeline)
		if err := b.AddServicePipeline(ctx, &pipeline, inputPipelineID,
			b.addOTLPReceiver(),
			b.addKymaStatsReceiver(),
			b.addMemoryLimiterProcessor(),
			b.addInputRoutingExporter(),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add input service pipeline: %w", err)
		}

		enrichmentPipelineID := formatEnrichmentServicePipelineID(&pipeline)
		if err := b.AddServicePipeline(ctx, &pipeline, enrichmentPipelineID,
			b.addEnrichmentRoutingReceiver(),
			b.addK8sAttributesProcessor(opts),
			b.addServiceEnrichmentProcessor(),
			b.addEnrichmentForwardExporter(),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add enrichment service pipeline: %w", err)
		}

		ouputPipelineID := formatOutputServicePipelineID(&pipeline)
		if err := b.AddServicePipeline(ctx, &pipeline, ouputPipelineID,
			b.addOutputRoutingReceiver(),
			b.addOutputForwardReceiver(),
			b.addSetInstrumentationScopeToKymaProcessor(opts),
			// Input source filters
			b.addDropIfRuntimeInputDisabledProcessor(),
			b.addDropIfPrometheusInputDisabledProcessor(),
			b.addDropIfIstioInputDisabledProcessor(),
			b.addDropEnvoyMetricsIfDisabledProcessor(),
			b.addDropIfOTLPInputDisabledProcessor(),
			// Namespace filters
			b.addRuntimeNamespaceFilterProcessor(),
			b.addPrometheusNamespaceFilterProcessor(),
			b.addIstioNamespaceFilterProcessor(),
			b.addOTLPNamespaceFilterProcessor(),
			// Runtime resource filters
			b.addDropRuntimePodMetricsProcessor(),
			b.addDropRuntimeContainerMetricsProcessor(),
			b.addDropRuntimeNodeMetricsProcessor(),
			b.addDropRuntimeVolumeMetricsProcessor(),
			b.addDropRuntimeDeploymentMetricsProcessor(),
			b.addDropRuntimeDaemonSetMetricsProcessor(),
			b.addDropRuntimeStatefulSetMetricsProcessor(),
			b.addDropRuntimeJobMetricsProcessor(),
			// Diagnostic metric filters
			b.addDropIstioDiagnosticMetricsProcessor(),
			b.addDropPrometheusDiagnosticMetricsProcessor(),

			b.addInsertClusterAttributesProcessor(opts),
			b.addDeleteSkipEnrichmentAttributeProcessor(),
			b.addDropKymaAttributesProcessor(),
			b.addUserDefinedTransformProcessor(),
			// Batch processor (always last)
			b.addBatchProcessor(),
			// OTLP exporter
			b.addOTLPExporter(queueSize),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add output service pipeline: %w", err)
		}
	}

	return b.Config, b.EnvVars, nil
}

// Helper functions

func formatInputMetricServicePipelineID(mp *telemetryv1alpha1.MetricPipeline) string {
	return fmt.Sprintf("metrics/%s-input", mp.Name)
}

func formatEnrichmentServicePipelineID(mp *telemetryv1alpha1.MetricPipeline) string {
	return fmt.Sprintf("metrics/%s-attributes-enrichment", mp.Name)
}

func formatOutputServicePipelineID(mp *telemetryv1alpha1.MetricPipeline) string {
	return fmt.Sprintf("metrics/%s-output", mp.Name)
}

func formatRoutingConnectorID(mp *telemetryv1alpha1.MetricPipeline) string {
	return fmt.Sprintf(common.ComponentIDRoutingConnector, mp.Name)
}

func formatForwardConnectorID(mp *telemetryv1alpha1.MetricPipeline) string {
	return fmt.Sprintf(common.ComponentIDForwardConnector, mp.Name)
}