package logagent

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

type BuilderConfig struct {
	GatewayOTLPServiceName types.NamespacedName
}

type Builder struct {
	Reader client.Reader
	Config BuilderConfig

	config  *common.Config
	envVars common.EnvVars
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
	b.config = &common.Config{
		Base:       common.BaseConfig(),
		Receivers:  make(map[string]any),
		Processors: make(map[string]any),
		Exporters:  make(map[string]any),
	}
	b.config.Extensions.FileStorage = &common.FileStorage{
		Directory: otelcollector.CheckpointVolumePath,
	}
	b.config.Service.Extensions = append(b.config.Service.Extensions, "file_storage")
	b.envVars = make(common.EnvVars)

	// Iterate over each LogPipeline CR and enrich the config with pipeline-specific components
	// Iterate over each LogPipeline CR and enrich the config with pipeline-specific components
	for i := range pipelines {
		if err := b.addServicePipeline(ctx, &pipelines[i],
			b.addFileLogReceiver(),
			b.addMemoryLimiterProcessor(),
			b.addSetInstrumentationScopeProcessor(opts),
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

	return b.config, b.envVars, nil
}

// Type aliases for common builder patterns
type buildComponentFunc = common.BuildComponentFunc[*telemetryv1alpha1.LogPipeline]
type componentConfigFunc = common.ComponentConfigFunc[*telemetryv1alpha1.LogPipeline]
type exporterComponentConfigFunc = common.ExporterComponentConfigFunc[*telemetryv1alpha1.LogPipeline]
type componentIDFunc = common.ComponentIDFunc[*telemetryv1alpha1.LogPipeline]

// staticComponentID returns a ComponentIDFunc that always returns the same component ID independent of the LogPipeline
var staticComponentID = common.StaticComponentID[*telemetryv1alpha1.LogPipeline]

func (b *Builder) addServicePipeline(ctx context.Context, lp *telemetryv1alpha1.LogPipeline, fs ...buildComponentFunc) error {
	// Add an empty pipeline to the config
	pipelineID := formatLogServicePipelineID(lp)
	b.config.Service.Pipelines[pipelineID] = common.Pipeline{}

	for _, f := range fs {
		if err := f(ctx, lp); err != nil {
			return fmt.Errorf("failed to add component: %w", err)
		}
	}

	return nil
}

// addReceiver creates a decorator for adding receivers
func (b *Builder) addReceiver(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return common.AddReceiver(
		b.config,
		componentIDFunc,
		configFunc,
		formatLogServicePipelineID,
	)
}

// addProcessor creates a decorator for adding processors
func (b *Builder) addProcessor(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return common.AddProcessor(
		b.config,
		componentIDFunc,
		configFunc,
		formatLogServicePipelineID,
	)
}

// addExporter creates a decorator for adding exporters
func (b *Builder) addExporter(componentIDFunc componentIDFunc, configFunc exporterComponentConfigFunc) buildComponentFunc {
	return common.AddExporter(
		b.config,
		b.envVars,
		componentIDFunc,
		configFunc,
		formatLogServicePipelineID,
	)
}

func (b *Builder) addFileLogReceiver() buildComponentFunc {
	return b.addReceiver(
		formatFileLogReceiverID,
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return fileLogReceiverConfig(lp)
		},
	)
}

//nolint:mnd // hardcoded values
func (b *Builder) addMemoryLimiterProcessor() buildComponentFunc {
	return b.addProcessor(
		staticComponentID(common.ComponentIDMemoryLimiterProcessor),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return &common.MemoryLimiter{
				CheckInterval:        "5s",
				LimitPercentage:      80,
				SpikeLimitPercentage: 25,
			}
		},
	)
}

func (b *Builder) addSetInstrumentationScopeProcessor(opts BuildOptions) buildComponentFunc {
	return b.addProcessor(
		staticComponentID(common.ComponentIDSetInstrumentationScopeProcessor),
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
	return b.addProcessor(
		staticComponentID(common.ComponentIDK8sAttributesProcessor),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return common.K8sAttributesProcessorConfig(opts.Enrichments)
		},
	)
}

func (b *Builder) addInsertClusterAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.addProcessor(
		staticComponentID(common.ComponentIDInsertClusterAttributesProcessor),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return common.InsertClusterAttributesProcessorConfig(opts.ClusterName, opts.ClusterUID, opts.CloudProvider)
		},
	)
}

func (b *Builder) addServiceEnrichmentProcessor() buildComponentFunc {
	return b.addProcessor(
		staticComponentID(common.ComponentIDServiceEnrichmentProcessor),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return common.ResolveServiceNameConfig()
		},
	)
}

func (b *Builder) addDropKymaAttributesProcessor() buildComponentFunc {
	return b.addProcessor(
		staticComponentID(common.ComponentIDDropKymaAttributesProcessor),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return common.DropKymaAttributesProcessorConfig()
		},
	)
}

func (b *Builder) addUserDefinedTransformProcessor() buildComponentFunc {
	return b.addProcessor(
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
	return b.addExporter(
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
	return fmt.Sprintf("filelog/%s", lp.Name)
}

func formatUserDefinedTransformProcessorID(lp *telemetryv1alpha1.LogPipeline) string {
	return fmt.Sprintf("transform/user-defined-%s", lp.Name)
}

func formatOTLPExporterID(lp *telemetryv1alpha1.LogPipeline) string {
	return common.ExporterID(lp.Spec.Output.OTLP.Protocol, lp.Name)
}
