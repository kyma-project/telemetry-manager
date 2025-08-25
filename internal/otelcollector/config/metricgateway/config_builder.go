package metricgateway

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
)

const (
	maxQueueSize = 256 // Maximum number of batches kept in memory before dropping
)

type Builder struct {
	Reader client.Reader

	config  *common.Config
	envVars common.EnvVars
}

type BuildOptions struct {
	GatewayNamespace            string
	InstrumentationScopeVersion string
	ClusterName                 string
	ClusterUID                  string
	CloudProvider               string
	Enrichments                 *operatorv1alpha1.EnrichmentSpec
}

type buildComponentFunc = common.BuildComponentFunc[*telemetryv1alpha1.MetricPipeline]
type componentIDFunc = common.ComponentIDFunc[*telemetryv1alpha1.MetricPipeline]
type componentConfigFunc = common.ComponentConfigFunc[*telemetryv1alpha1.MetricPipeline]
type exporterComponentConfigFunc = common.ExporterComponentConfigFunc[*telemetryv1alpha1.MetricPipeline]

var staticComponentID = common.StaticComponentID[*telemetryv1alpha1.MetricPipeline]

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.MetricPipeline, opts BuildOptions) (*common.Config, common.EnvVars, error) {
	b.config = &common.Config{
		Base:       common.BaseConfig(common.WithK8sLeaderElector("serviceAccount", "telemetry-metric-gateway-kymastats", opts.GatewayNamespace)),
		Receivers:  make(map[string]any),
		Processors: make(map[string]any),
		Exporters:  make(map[string]any),
		Connectors: make(map[string]any),
	}
	b.envVars = make(common.EnvVars)

	// Iterate over each MetricPipeline CR and enrich the config with pipeline-specific components
	queueSize := maxQueueSize / len(pipelines)

	for i := range pipelines {
		if err := b.addInputServicePipeline(ctx, &pipelines[i],
			b.addOTLPReceiver(),
			b.addKymaStatsReceiver(),
			b.addMemoryLimiterProcessor(),
			b.addInputRoutingExporter(),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add input service pipeline: %w", err)
		}

		if err := b.addEnrichmentServicePipeline(ctx, &pipelines[i],
			b.addEnrichmentRoutingReceiver(),
			b.addK8sAttributesProcessor(opts),
			b.addServiceEnrichmentProcessor(),
			b.addEnrichmentForwardExporter(),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add enrichment service pipeline: %w", err)
		}

		if err := b.addOutputServicePipeline(ctx, &pipelines[i],
			b.addOutputRoutingReceiver(),
			b.addOutputForwardReceiver(),
			b.addSetInstrumentationScopeProcessor(opts),
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

	return b.config, b.envVars, nil
}

// Helper functions

func enrichmentRoutingConnectorConfig(mp *telemetryv1alpha1.MetricPipeline) common.RoutingConnector {
	enrichmentPipelineID := formatEnrichmentServicePipelineID(mp)
	outputPipelineID := formatOutputServicePipelineID(mp)

	return common.RoutingConnector{
		DefaultPipelines: []string{enrichmentPipelineID},
		ErrorMode:        "ignore",
		Table: []common.RoutingConnectorTableEntry{
			{
				Statement: fmt.Sprintf("route() where attributes[\"%s\"] == \"true\"", common.SkipEnrichmentAttribute),
				Pipelines: []string{outputPipelineID},
			},
		},
	}
}

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
