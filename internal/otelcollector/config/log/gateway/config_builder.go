package gateway

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

const (
	maxQueueSize = 256 // Maximum number of batches kept in memory before dropping
)

type Builder struct {
	Reader client.Reader
}

type BuildOptions struct {
	ClusterName   string
	ClusterUID    string
	CloudProvider string
	Enrichments   *operatorv1alpha1.EnrichmentSpec
	ModuleVersion string
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.LogPipeline, opts BuildOptions) (*Config, otlpexporter.EnvVars, error) {
	cfg := newConfig(opts)

	// Iterate over each LogPipeline CR and enrich the config with pipeline-specific components
	queueSize := maxQueueSize / len(pipelines)
	envVars := make(otlpexporter.EnvVars)

	for i := range pipelines {
		pipeline := pipelines[i]

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

		// Assemble the service pipeline for this LogPipeline
		pipelineID := fmt.Sprintf("logs/%s", pipeline.Name)
		cfg.Service.Pipelines[pipelineID] = servicePipelineConfig(&pipeline)
	}

	return cfg, envVars, nil
}

func receiversConfig() Receivers {
	return Receivers{
		OTLP: config.OTLPReceiver{
			Protocols: config.ReceiverProtocols{
				HTTP: config.Endpoint{
					Endpoint: fmt.Sprintf("${%s}:%d", config.EnvVarCurrentPodIP, ports.OTLPHTTP),
				},
				GRPC: config.Endpoint{
					Endpoint: fmt.Sprintf("${%s}:%d", config.EnvVarCurrentPodIP, ports.OTLPGRPC),
				},
			},
		},
	}
}
