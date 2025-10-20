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

	queueSize := common.BatchingMaxQueueSize / len(pipelines)

	for _, pipeline := range pipelines {
		pipelineID := formatInputMetricServicePipelineID(&pipeline)
		if err := b.AddServicePipeline(ctx, &pipeline, pipelineID,
			b.addOTLPReceiver(),
			b.addKymaStatsReceiver(),
			b.addMemoryLimiterProcessor(),
			b.addK8sAttributesProcessor(opts),
			b.addServiceEnrichmentProcessor(),
			b.addSetInstrumentationScopeToKymaProcessor(opts),
			// Input source filters
			b.addDropIfOTLPInputDisabledProcessor(),
			// Namespace filters
			b.addOTLPNamespaceFilterProcessor(),
			b.addInsertClusterAttributesProcessor(opts),
			b.addDropKymaAttributesProcessor(),
			b.addUserDefinedTransformProcessor(),
			b.addUserDefinedFilterProcessor(),
			// Batch processor (always last)
			b.addBatchProcessor(),
			// OTLP exporter
			b.addOTLPExporter(queueSize),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add input service pipeline: %w", err)
		}
	}

	return b.Config, b.EnvVars, nil
}

// Helper functions

func formatInputMetricServicePipelineID(mp *telemetryv1alpha1.MetricPipeline) string {
	return fmt.Sprintf("metrics/%s-input", mp.Name)
}
