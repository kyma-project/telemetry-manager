package logagent

import (
	"context"
	"fmt"
	"path/filepath"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

const checkpointVolumePathSubdir = "telemetry-log-agent/file-log-receiver"

type buildComponentFunc = common.BuildComponentFunc[*telemetryv1beta1.LogPipeline]

type Builder struct {
	common.ComponentBuilder[*telemetryv1beta1.LogPipeline]

	Reader           client.Reader
	collectAgentLogs bool
}

type BuildOptions struct {
	Cluster                     common.ClusterOptions
	InstrumentationScopeVersion string
	AgentNamespace              string
	Enrichments                 *operatorv1beta1.EnrichmentSpec
	// ServiceEnrichment specifies the service enrichment strategy to be used (temporary)
	ServiceEnrichment string
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1beta1.LogPipeline, opts BuildOptions) (*common.Config, common.EnvVars, error) {
	b.Config = common.NewConfig()
	b.AddExtension(common.ComponentIDFileStorageExtension, &common.FileStorageExtension{
		CreateDirectory: true,
		Directory:       filepath.Join(otelcollector.CheckpointVolumePath, checkpointVolumePathSubdir),
	}, nil)
	b.EnvVars = make(common.EnvVars)

	for _, pipeline := range pipelines {
		pipelineID := formatLogServicePipelineID(&pipeline)

		if shouldEnableOAuth2(&pipeline) {
			if err := b.addOAuth2Extension(ctx, &pipeline); err != nil {
				return nil, nil, err
			}
		}

		if err := b.AddServicePipeline(ctx, &pipeline, pipelineID,
			b.addFileLogReceiver(),
			b.addMemoryLimiterProcessor(),
			b.addSetInstrumentationScopeToRuntimeProcessor(opts),
			b.addDropUnknownServiceNameProcessor(opts),
			b.addK8sAttributesProcessor(opts),
			b.addInsertClusterAttributesProcessor(opts),
			b.addServiceEnrichmentProcessor(opts),
			// Kyma attributes are dropped before user-defined transform and filter processors
			// to prevent user access to internal attributes.
			b.addDropKymaAttributesProcessor(),
			b.addUserDefinedTransformProcessor(),
			b.addUserDefinedFilterProcessor(),
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
		func(lp *telemetryv1beta1.LogPipeline) any {
			return fileLogReceiverConfig(lp, b.collectAgentLogs)
		},
	)
}

//nolint:mnd // hardcoded values
func (b *Builder) addMemoryLimiterProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDMemoryLimiterProcessor),
		func(lp *telemetryv1beta1.LogPipeline) any {
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
		func(lp *telemetryv1beta1.LogPipeline) any {
			return common.LogTransformProcessorConfig([]common.TransformProcessorStatements{{
				Statements: []string{
					fmt.Sprintf("set(scope.version, %q)", opts.InstrumentationScopeVersion),
					fmt.Sprintf("set(scope.name, %q)", common.InstrumentationScopeRuntime),
				},
			}})
		},
	)
}

func (b *Builder) addDropUnknownServiceNameProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropUnknownServiceNameProcessor),
		func(tp *telemetryv1beta1.LogPipeline) any {
			if opts.ServiceEnrichment != commonresources.AnnotationValueTelemetryServiceEnrichmentOtel {
				return nil // Kyma legacy enrichment selected, skip this processor
			}

			return common.LogTransformProcessorConfig(common.DropUnknownServiceNameProcessorStatements())
		},
	)
}

func (b *Builder) addK8sAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDK8sAttributesProcessor),
		func(lp *telemetryv1beta1.LogPipeline) any {
			useOTelServiceEnrichment := opts.ServiceEnrichment == commonresources.AnnotationValueTelemetryServiceEnrichmentOtel
			return common.K8sAttributesProcessorConfig(opts.Enrichments, useOTelServiceEnrichment)
		},
	)
}

func (b *Builder) addInsertClusterAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDInsertClusterAttributesProcessor),
		func(lp *telemetryv1beta1.LogPipeline) any {
			transformStatements := common.InsertClusterAttributesProcessorStatements(opts.Cluster)
			return common.LogTransformProcessorConfig(transformStatements)
		},
	)
}

func (b *Builder) addServiceEnrichmentProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDServiceEnrichmentProcessor),
		func(lp *telemetryv1beta1.LogPipeline) any {
			if opts.ServiceEnrichment == commonresources.AnnotationValueTelemetryServiceEnrichmentOtel {
				return nil // OTel service enrichment selected, skip this processor
			}

			return common.ResolveServiceNameConfig()
		},
	)
}

func (b *Builder) addDropKymaAttributesProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropKymaAttributesProcessor),
		func(lp *telemetryv1beta1.LogPipeline) any {
			transformStatements := common.DropKymaAttributesProcessorStatements()
			return common.LogTransformProcessorConfig(transformStatements)
		},
	)
}

func (b *Builder) addUserDefinedTransformProcessor() buildComponentFunc {
	return b.AddProcessor(
		formatUserDefinedTransformProcessorID,
		func(lp *telemetryv1beta1.LogPipeline) any {
			if len(lp.Spec.Transforms) == 0 {
				return nil // No transforms, no processor needed
			}

			transformStatements := common.TransformSpecsToProcessorStatements(lp.Spec.Transforms)

			return common.LogTransformProcessorConfig(transformStatements)
		},
	)
}

func (b *Builder) addUserDefinedFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		formatUserDefinedFilterProcessorID,
		func(lp *telemetryv1beta1.LogPipeline) any {
			if lp.Spec.Filters == nil {
				return nil // No filters, no processor needed
			}

			return common.FilterSpecsToLogFilterProcessorConfig(lp.Spec.Filters)
		},
	)
}

func (b *Builder) addOTLPExporter() buildComponentFunc {
	return b.AddExporter(
		formatOTLPExporterID,
		func(ctx context.Context, lp *telemetryv1beta1.LogPipeline) (any, common.EnvVars, error) {
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

func (b *Builder) addOAuth2Extension(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline) error {
	oauth2ExtensionID := common.OAuth2ExtensionID(pipeline.Name)

	oauth2ExtensionConfig, oauth2ExtensionEnvVars, err := common.NewOAuth2ExtensionConfigBuilder(
		b.Reader,
		pipeline.Spec.Output.OTLP.Authentication.OAuth2,
		pipeline.Name,
		common.SignalTypeTrace,
	).OAuth2ExtensionConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to build OAuth2 extension for pipeline %s: %w", pipeline.Name, err)
	}

	b.AddExtension(oauth2ExtensionID, oauth2ExtensionConfig, oauth2ExtensionEnvVars)

	return nil
}

func shouldEnableOAuth2(tp *telemetryv1beta1.LogPipeline) bool {
	return tp.Spec.Output.OTLP.Authentication != nil && tp.Spec.Output.OTLP.Authentication.OAuth2 != nil
}

func formatLogServicePipelineID(lp *telemetryv1beta1.LogPipeline) string {
	return fmt.Sprintf("logs/%s", lp.Name)
}

func formatFileLogReceiverID(lp *telemetryv1beta1.LogPipeline) string {
	return fmt.Sprintf(common.ComponentIDFileLogReceiver, lp.Name)
}

func formatUserDefinedTransformProcessorID(lp *telemetryv1beta1.LogPipeline) string {
	return fmt.Sprintf(common.ComponentIDUserDefinedTransformProcessor, lp.Name)
}

func formatUserDefinedFilterProcessorID(lp *telemetryv1beta1.LogPipeline) string {
	return fmt.Sprintf(common.ComponentIDUserDefinedFilterProcessor, lp.Name)
}

func formatOTLPExporterID(lp *telemetryv1beta1.LogPipeline) string {
	return common.ExporterID(lp.Spec.Output.OTLP.Protocol, lp.Name)
}
