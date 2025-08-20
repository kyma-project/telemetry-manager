package loggateway

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
)

const (
	maxQueueSize = 256 // Maximum number of batches kept in memory before dropping
)

type Builder struct {
	Reader client.Reader

	config  *Config
	envVars common.EnvVars
}

type BuildOptions struct {
	ClusterName   string
	ClusterUID    string
	CloudProvider string
	Enrichments   *operatorv1alpha1.EnrichmentSpec
	ModuleVersion string
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.LogPipeline, opts BuildOptions) (*Config, common.EnvVars, error) {
	b.config = &Config{
		Base:       common.BaseConfig(),
		Receivers:  receiversConfig(),
		Processors: processorsConfig(opts, pipelines),
		Exporters:  make(Exporters),
	}
	b.envVars = make(common.EnvVars)

	// Iterate over each LogPipeline CR and enrich the config with pipeline-specific components
	queueSize := maxQueueSize / len(pipelines)

	for i := range pipelines {
		if err := b.addServicePipeline(ctx, &pipelines[i],
			b.addOTLPReceiver(),
			b.addMemoryLimiterProcessor(),
			b.addSetObsTimeIfZeroProcessor(),
			b.addK8sAttributesProcessor(opts),
			b.addIstioNoiseFilterProcessor(),
			b.addDropIfInputSourceOTLPProcessor(),
			b.addNamespaceFilterProcessor(),
			b.addInsertClusterAttributesProcessor(opts),
			b.addServiceEnrichmentProcessor(),
			b.addDropKymaAttributesProcessor(),
			b.addIstioEnrichmentProcessor(opts),
			b.addUserDefinedTransformProcessor(),
			b.addBatchProcessor(),
			b.addOTLPExporter(queueSize),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add service pipeline: %w", err)
		}
	}

	return b.config, b.envVars, nil
}

// buildComponentFunc defines a function type for building components in the telemetry configuration
type buildComponentFunc func(ctx context.Context, lp *telemetryv1alpha1.LogPipeline) error

// componentConfigFunc creates the configuration for a component (receiver or processor)
type componentConfigFunc func(lp *telemetryv1alpha1.LogPipeline) any

// exporterComponentConfigFunc creates the configuration for an exporter component
// creating exporters is different from receivers and processors, as it makes an API server call to resolve the reference secrets
// and returns the configuration along with environment variables needed for the exporter
type exporterComponentConfigFunc func(ctx context.Context, lp *telemetryv1alpha1.LogPipeline) (any, common.EnvVars, error)

// componentIDFunc determines the ID of a component
type componentIDFunc func(*telemetryv1alpha1.LogPipeline) string

// staticComponentID returns a ComponentIDFunc that always returns the same component ID independent of the LogPipeline
func staticComponentID(componentID string) componentIDFunc {
	return func(*telemetryv1alpha1.LogPipeline) string {
		return componentID
	}
}

func (b *Builder) addServicePipeline(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline, fs ...buildComponentFunc) error {
	// Add an empty pipeline to the config
	pipelineID := formatLogPipelineID(pipeline.Name)
	b.config.Service.Pipelines[pipelineID] = common.Pipeline{}

	for _, f := range fs {
		if err := f(ctx, pipeline); err != nil {
			return fmt.Errorf("failed to add component: %w", err)
		}
	}

	return nil
}

// withReceiver creates a decorator for adding receivers
func (b *Builder) withReceiver(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return func(ctx context.Context, lp *telemetryv1alpha1.LogPipeline) error {
		config := configFunc(lp)
		if config == nil {
			// If no config is provided, skip adding the receiver
			return nil
		}

		componentID := componentIDFunc(lp)
		if componentID == "otlp" {
			// OTLP receiver is already configured in receiversConfig()
			// Just add it to the pipeline
		}

		pipelineID := formatLogPipelineID(lp.Name)
		pipeline := b.config.Service.Pipelines[pipelineID]
		pipeline.Receivers = append(pipeline.Receivers, componentID)
		b.config.Service.Pipelines[pipelineID] = pipeline

		return nil
	}
}

// withProcessor creates a decorator for adding processors
func (b *Builder) withProcessor(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return func(ctx context.Context, lp *telemetryv1alpha1.LogPipeline) error {
		config := configFunc(lp)
		if config == nil {
			// If no config is provided, skip adding the processor
			return nil
		}

		componentID := componentIDFunc(lp)

		// Check if this is a dynamic processor that needs to be added to the Dynamic map
		if isDynamicProcessor(componentID) {
			if b.config.Processors.Dynamic == nil {
				b.config.Processors.Dynamic = make(map[string]any)
			}
			if _, found := b.config.Processors.Dynamic[componentID]; !found {
				b.config.Processors.Dynamic[componentID] = config
			}
		}
		// Static processors are already configured in processorsConfig()

		pipelineID := formatLogPipelineID(lp.Name)
		pipeline := b.config.Service.Pipelines[pipelineID]
		pipeline.Processors = append(pipeline.Processors, componentID)
		b.config.Service.Pipelines[pipelineID] = pipeline

		return nil
	}
}

// withExporter creates a decorator for adding exporters
func (b *Builder) withExporter(componentIDFunc componentIDFunc, configFunc exporterComponentConfigFunc) buildComponentFunc {
	return func(ctx context.Context, lp *telemetryv1alpha1.LogPipeline) error {
		config, envVars, err := configFunc(ctx, lp)
		if err != nil {
			return fmt.Errorf("failed to create exporter config: %w", err)
		}

		if config == nil {
			// If no config is provided, skip adding the exporter
			return nil
		}

		componentID := componentIDFunc(lp)
		b.config.Exporters[componentID] = Exporter{OTLP: config.(*common.OTLPExporter)}
		maps.Copy(b.envVars, envVars)

		pipelineID := formatLogPipelineID(lp.Name)
		pipeline := b.config.Service.Pipelines[pipelineID]
		pipeline.Exporters = append(pipeline.Exporters, componentID)
		b.config.Service.Pipelines[pipelineID] = pipeline

		return nil
	}
}

func (b *Builder) addOTLPReceiver() buildComponentFunc {
	return b.withReceiver(
		staticComponentID("otlp"),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			// OTLP receiver config is already set in receiversConfig()
			return &common.OTLPReceiver{}
		},
	)
}

func (b *Builder) addMemoryLimiterProcessor() buildComponentFunc {
	return b.withProcessor(
		staticComponentID("memory_limiter"),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			// Memory limiter config is already set in processorsConfig()
			return &common.MemoryLimiter{}
		},
	)
}

func (b *Builder) addSetObsTimeIfZeroProcessor() buildComponentFunc {
	return b.withProcessor(
		staticComponentID("transform/set-observed-time-if-zero"),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			// Set observed time config is already set in processorsConfig()
			return &common.TransformProcessor{}
		},
	)
}

func (b *Builder) addK8sAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.withProcessor(
		staticComponentID("k8sattributes"),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			// K8s attributes config is already set in processorsConfig()
			return &common.K8sAttributesProcessor{}
		},
	)
}

func (b *Builder) addIstioNoiseFilterProcessor() buildComponentFunc {
	return b.withProcessor(
		staticComponentID("istio_noise_filter"),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			// Istio noise filter config is already set in processorsConfig()
			return &common.IstioNoiseFilterProcessor{}
		},
	)
}

func (b *Builder) addDropIfInputSourceOTLPProcessor() buildComponentFunc {
	return b.withProcessor(
		staticComponentID("filter/drop-if-input-source-otlp"),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			if logpipelineutils.IsOTLPInputEnabled(lp.Spec.Input) {
				return nil // Skip this processor if OTLP input is enabled
			}
			// Check if processor was already configured in processorsConfig()
			if b.config.Processors.DropIfInputSourceOTLP != nil {
				return b.config.Processors.DropIfInputSourceOTLP
			}
			return nil
		},
	)
}

func (b *Builder) addNamespaceFilterProcessor() buildComponentFunc {
	return b.withProcessor(
		formatNamespaceFilterID,
		func(lp *telemetryv1alpha1.LogPipeline) any {
			otlpInput := lp.Spec.Input.OTLP
			if otlpInput == nil || otlpInput.Disabled || !shouldFilterByNamespace(otlpInput.Namespaces) {
				return nil // No namespace filter needed
			}

			return namespaceFilterProcessorConfig(otlpInput.Namespaces)
		},
	)
}

func (b *Builder) addInsertClusterAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.withProcessor(
		staticComponentID("resource/insert-cluster-attributes"),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			// Insert cluster attributes config is already set in processorsConfig()
			return &common.ResourceProcessor{}
		},
	)
}

func (b *Builder) addServiceEnrichmentProcessor() buildComponentFunc {
	return b.withProcessor(
		staticComponentID("service_enrichment"),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			// Service enrichment config is already set in processorsConfig()
			return &common.ServiceEnrichmentProcessor{}
		},
	)
}

func (b *Builder) addDropKymaAttributesProcessor() buildComponentFunc {
	return b.withProcessor(
		staticComponentID("resource/drop-kyma-attributes"),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			// Drop Kyma attributes config is already set in processorsConfig()
			return &common.ResourceProcessor{}
		},
	)
}

func (b *Builder) addIstioEnrichmentProcessor(opts BuildOptions) buildComponentFunc {
	return b.withProcessor(
		staticComponentID("istio_enrichment"),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			// Istio enrichment config is already set in processorsConfig()
			return &IstioEnrichmentProcessor{}
		},
	)
}

func (b *Builder) addUserDefinedTransformProcessor() buildComponentFunc {
	return b.withProcessor(
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

func (b *Builder) addBatchProcessor() buildComponentFunc {
	return b.withProcessor(
		staticComponentID("batch"),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			// Batch processor config is already set in processorsConfig()
			return &common.BatchProcessor{}
		},
	)
}

func (b *Builder) addOTLPExporter(queueSize int) buildComponentFunc {
	return b.withExporter(
		formatOTLPExporterID,
		func(ctx context.Context, lp *telemetryv1alpha1.LogPipeline) (any, common.EnvVars, error) {
			otlpExporterBuilder := common.NewOTLPExporterConfigBuilder(
				b.Reader,
				lp.Spec.Output.OTLP,
				lp.Name,
				queueSize,
				common.SignalTypeLog,
			)

			otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.OTLPExporterConfig(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create otlp exporter config: %w", err)
			}

			return otlpExporterConfig, otlpExporterEnvVars, nil
		},
	)
}

// Helper functions

func receiversConfig() Receivers {
	return Receivers{
		OTLP: common.OTLPReceiver{
			Protocols: common.ReceiverProtocols{
				HTTP: common.Endpoint{
					Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OTLPHTTP),
				},
				GRPC: common.Endpoint{
					Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OTLPGRPC),
				},
			},
		},
	}
}

func isDynamicProcessor(componentID string) bool {
	// Dynamic processors are those with pipeline-specific IDs
	switch {
	case strings.HasPrefix(componentID, "filter/") && componentID != "filter/drop-if-input-source-otlp":
		return true
	case strings.HasPrefix(componentID, "transform/user-defined-"):
		return true
	default:
		return false
	}
}

func formatLogPipelineID(pipelineName string) string {
	return fmt.Sprintf("logs/%s", pipelineName)
}

func shouldFilterByNamespace(namespaceSelector *telemetryv1alpha1.NamespaceSelector) bool {
	return namespaceSelector != nil && (len(namespaceSelector.Include) > 0 || len(namespaceSelector.Exclude) > 0)
}

func formatNamespaceFilterID(lp *telemetryv1alpha1.LogPipeline) string {
	return fmt.Sprintf("filter/%s-filter-by-namespace", lp.Name)
}

func formatUserDefinedTransformProcessorID(lp *telemetryv1alpha1.LogPipeline) string {
	return fmt.Sprintf("transform/user-defined-%s", lp.Name)
}

func formatOTLPExporterID(pipeline *telemetryv1alpha1.LogPipeline) string {
	return common.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
}
