package agent

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
)

type BuilderConfig struct {
	GatewayOTLPServiceName types.NamespacedName
}
type Builder struct {
	Reader client.Reader
	Config BuilderConfig
}

// Currently the queue is disabled. So set the size to 0
const queueSize = 0

type BuildOptions struct {
	InstrumentationScopeVersion string
	AgentNamespace              string
	ClusterName                 string
	ClusterUID                  string
	CloudProvider               string
	Enrichments                 *operatorv1alpha1.EnrichmentSpec
}

func (b *Builder) Build(ctx context.Context, logPipelines []telemetryv1alpha1.LogPipeline, opts BuildOptions) (*Config, otlpexporter.EnvVars, error) {
	cfg := newConfig(opts)

	// Iterate over each LogPipeline CR and enrich the config with pipeline-specific components
	envVars := make(otlpexporter.EnvVars)

	for i := range logPipelines {
		pipeline := logPipelines[i]

		otlpExporterBuilder := otlpexporter.NewConfigBuilder(
			b.Reader,
			pipeline.Spec.Output.OTLP,
			pipeline.Name,
			queueSize,
			otlpexporter.SignalTypeLog,
		)
		if err := cfg.addLogPipelineComponents(ctx, otlpExporterBuilder, &pipeline, envVars); err != nil {
			return nil, nil, err
		}

		pipelineID := fmt.Sprintf("logs/%s", pipeline.Name)
		cfg.Service.Pipelines[pipelineID] = pipelineConfig(fmt.Sprintf("filelog/%s", pipeline.Name), otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name))
	}

	return cfg, envVars, nil
}

// Each pipeline will have one receiver and one exporter
func pipelineConfig(receiverID, exporterID string) config.Pipeline {
	return config.Pipeline{
		Receivers: []string{receiverID},
		Processors: []string{
			"memory_limiter",
			"transform/set-instrumentation-scope-runtime",
			"k8sattributes",
			"resource/insert-cluster-attributes",
			"service_enrichment",
			"resource/drop-kyma-attributes",
		},
		Exporters: []string{exporterID},
	}
}
