package logagent

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

type buildComponentFunc = common.BuildComponentFunc[*telemetryv1alpha1.LogPipeline]

type Builder struct {
	common.ComponentBuilder[*telemetryv1alpha1.LogPipeline]

	Reader client.Reader
}

type BuildOptions struct {
	InstrumentationScopeVersion string
	AgentNamespace              string
	ClusterName                 string
	ClusterUID                  string
	CloudProvider               string
	Enrichments                 *operatorv1alpha1.EnrichmentSpec
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.LogPipeline, opts BuildOptions) (*common.Config, common.EnvVars, error) {
	b.Config = &common.Config{
		Base:       common.BaseConfig(),
		Receivers:  make(map[string]any),
		Processors: make(map[string]any),
		Exporters:  make(map[string]any),
	}
	b.Config.Extensions.FileStorage = &common.FileStorage{
		Directory: otelcollector.CheckpointVolumePath,
	}
	b.Config.Service.Extensions = append(b.Config.Service.Extensions, "file_storage")
	b.EnvVars = make(common.EnvVars)

	for _, pipeline := range pipelines {
		pipelineID := formatLogServicePipelineID(&pipeline)
		if err := b.AddServicePipeline(ctx, &pipeline, pipelineID,
			b.addFileLogReceiver(),
			b.addMemoryLimiterProcessor(),
			b.addSetInstrumentationScopeToRuntimeProcessor(opts),
			b.addK8sAttributesProcessor(opts),
			b.addInsertClusterAttributesProcessor(opts),
			b.addServiceEnrichmentProcessor(),
			b.addDropKymaAttributesProcessor(),
			b.addUserDefinedTransformProcessor(),
			b.addOTLPExporter(),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add service pipeline: %w", err)
		}
	}

	return b.Config, b.EnvVars, nil
}

func (b *Builder) addFileLogReceiver() buildComponentFunc {
	return b.AddReceiver(
		formatFileLogReceiverID,
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return fileLogReceiverConfig(lp)
		},
	)
}

//nolint:mnd // hardcoded values
func (b *Builder) addMemoryLimiterProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDMemoryLimiterProcessor),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return &common.MemoryLimiter{
				CheckInterval:        "5s",
				LimitPercentage:      80,
				SpikeLimitPercentage: 25,
			}
		},
	)
}

func (b *Builder) addSetInstrumentationScopeToRuntimeProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDSetInstrumentationScopeRuntimeProcessor),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return common.LogTransformProcessorConfig([]common.TransformProcessorStatements{{
				Statements: []string{
					fmt.Sprintf("set(scope.version, %q)", opts.InstrumentationScopeVersion),
					fmt.Sprintf("set(scope.name, %q)", common.InstrumentationScopeRuntime),
				},
			}})
		},
	)
}

func (b *Builder) addK8sAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDK8sAttributesProcessor),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return common.K8sAttributesProcessorConfig(opts.Enrichments)
		},
	)
}

func (b *Builder) addInsertClusterAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDInsertClusterAttributesProcessor),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return common.InsertClusterAttributesProcessorConfig(opts.ClusterName, opts.ClusterUID, opts.CloudProvider)
		},
	)
}

func (b *Builder) addServiceEnrichmentProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDServiceEnrichmentProcessor),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return common.ResolveServiceNameConfig()
		},
	)
}

func (b *Builder) addDropKymaAttributesProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropKymaAttributesProcessor),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return common.DropKymaAttributesProcessorConfig()
		},
	)
}

func (b *Builder) addUserDefinedTransformProcessor() buildComponentFunc {
	return b.AddProcessor(
		formatUserDefinedTransformProcessorID,
		func(lp *telemetryv1alpha1.LogPipeline) any {
			if len(lp.Spec.Transforms) == 0 {
				return nil // No transforms, no processor needed
			}

			transformStatements := common.TransformSpecsToProcessorStatements(lp.Spec.Transforms)
			transformProcessor := common.LogTransformProcessorConfig(transformStatements)

			return transformProcessor
		},
	)
}

func (b *Builder) addOTLPExporter() buildComponentFunc {
	return b.AddExporter(
		formatOTLPExporterID,
		func(ctx context.Context, lp *telemetryv1alpha1.LogPipeline) (any, common.EnvVars, error) {
			otlpExporterBuilder := common.NewOTLPExporterConfigBuilder(
				b.Reader,
				lp.Spec.Output.OTLP,
				lp.Name,
				0, // queue size is set to 0 for now, as the queue is disabled
				common.SignalTypeLog,
			)

			return otlpExporterBuilder.OTLPExporterConfig(ctx)
		},
	)
}

func formatLogServicePipelineID(lp *telemetryv1alpha1.LogPipeline) string {
	return fmt.Sprintf("logs/%s", lp.Name)
}

func formatFileLogReceiverID(lp *telemetryv1alpha1.LogPipeline) string {
	return fmt.Sprintf(common.ComponentIDFileLogReceiver, lp.Name)
}

func formatUserDefinedTransformProcessorID(lp *telemetryv1alpha1.LogPipeline) string {
	return fmt.Sprintf(common.ComponentIDUserDefinedTransformProcessor, lp.Name)
}

func formatOTLPExporterID(lp *telemetryv1alpha1.LogPipeline) string {
	return common.ExporterID(lp.Spec.Output.OTLP.Protocol, lp.Name)
}
