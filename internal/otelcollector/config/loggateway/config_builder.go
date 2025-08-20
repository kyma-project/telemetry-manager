package loggateway

import (
	"context"
	"fmt"

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

	config  *common.Config
	envVars common.EnvVars
}

type BuildOptions struct {
	ClusterName   string
	ClusterUID    string
	CloudProvider string
	Enrichments   *operatorv1alpha1.EnrichmentSpec
	ModuleVersion string
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.LogPipeline, opts BuildOptions) (*common.Config, common.EnvVars, error) {
	b.config = &common.Config{
		Base:       common.BaseConfig(),
		Receivers:  make(map[string]any),
		Processors: make(map[string]any),
		Exporters:  make(map[string]any),
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

	return common.AddServicePipeline(
		ctx,
		lp,
		pipelineID,
		func(id string) {
			b.config.Service.Pipelines[id] = common.Pipeline{}
		},
		fs...,
	)
}

// withReceiver creates a decorator for adding receivers
func (b *Builder) withReceiver(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return common.WithReceiver[*telemetryv1alpha1.LogPipeline, any](
		componentIDFunc,
		configFunc,
		b.config.Receivers,
		b.config.Service.Pipelines,
		formatLogServicePipelineID,
	)
}

// withProcessor creates a decorator for adding processors
func (b *Builder) withProcessor(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return common.WithProcessor[*telemetryv1alpha1.LogPipeline, any](
		componentIDFunc,
		configFunc,
		b.config.Processors,
		b.config.Service.Pipelines,
		formatLogServicePipelineID,
	)
}

// withExporter creates a decorator for adding exporters
func (b *Builder) withExporter(componentIDFunc componentIDFunc, configFunc exporterComponentConfigFunc) buildComponentFunc {
	return common.WithExporter[*telemetryv1alpha1.LogPipeline, any](
		componentIDFunc,
		configFunc,
		b.config.Exporters,
		b.config.Service.Pipelines,
		b.envVars,
		formatLogServicePipelineID,
	)
}

func (b *Builder) addOTLPReceiver() buildComponentFunc {
	return b.withReceiver(
		staticComponentID("otlp"),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return &common.OTLPReceiver{
				Protocols: common.ReceiverProtocols{
					HTTP: common.Endpoint{
						Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OTLPHTTP),
					},
					GRPC: common.Endpoint{
						Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OTLPGRPC),
					},
				},
			}
		},
	)
}

//nolint:mnd // hardcoded values
func (b *Builder) addMemoryLimiterProcessor() buildComponentFunc {
	return b.withProcessor(
		staticComponentID("memory_limiter"),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return &common.MemoryLimiter{
				CheckInterval:        "1s",
				LimitPercentage:      75,
				SpikeLimitPercentage: 15,
			}
		},
	)
}

func (b *Builder) addSetObsTimeIfZeroProcessor() buildComponentFunc {
	return b.withProcessor(
		staticComponentID("transform/set-observed-time-if-zero"),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return common.LogTransformProcessorConfig([]common.TransformProcessorStatements{{
				Conditions: []string{"log.observed_time_unix_nano == 0"},
				Statements: []string{"set(log.observed_time, Now())"},
			}})
		},
	)
}

func (b *Builder) addK8sAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.withProcessor(
		staticComponentID("k8sattributes"),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return common.K8sAttributesProcessorConfig(opts.Enrichments)
		},
	)
}

func (b *Builder) addIstioNoiseFilterProcessor() buildComponentFunc {
	return b.withProcessor(
		staticComponentID("istio_noise_filter"),
		func(lp *telemetryv1alpha1.LogPipeline) any {
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

			return dropIfInputSourceOTLPProcessorConfig()
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
			return common.InsertClusterAttributesProcessorConfig(opts.ClusterName, opts.ClusterUID, opts.CloudProvider)
		},
	)
}

func (b *Builder) addServiceEnrichmentProcessor() buildComponentFunc {
	return b.withProcessor(
		staticComponentID("service_enrichment"),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return common.ResolveServiceNameConfig()
		},
	)
}

func (b *Builder) addDropKymaAttributesProcessor() buildComponentFunc {
	return b.withProcessor(
		staticComponentID("resource/drop-kyma-attributes"),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return common.DropKymaAttributesProcessorConfig()
		},
	)
}

func (b *Builder) addIstioEnrichmentProcessor(opts BuildOptions) buildComponentFunc {
	return b.withProcessor(
		staticComponentID("istio_enrichment"),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return istioEnrichmentProcessorConfig(opts)
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

//nolint:mnd // hardcoded values
func (b *Builder) addBatchProcessor() buildComponentFunc {
	return b.withProcessor(
		staticComponentID("batch"),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return &common.BatchProcessor{
				SendBatchSize:    512,
				Timeout:          "10s",
				SendBatchMaxSize: 512,
			}
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

func istioEnrichmentProcessorConfig(opts BuildOptions) *IstioEnrichmentProcessor {
	return &IstioEnrichmentProcessor{
		ScopeVersion: opts.ModuleVersion,
	}
}

func namespaceFilterProcessorConfig(namespaceSelector *telemetryv1alpha1.NamespaceSelector) *FilterProcessor {
	var filterExpressions []string

	if len(namespaceSelector.Exclude) > 0 {
		namespacesConditions := namespacesConditions(namespaceSelector.Exclude)

		// Drop logs if the excluded namespaces are matched
		excludeNamespacesExpr := common.JoinWithOr(namespacesConditions...)
		filterExpressions = append(filterExpressions, excludeNamespacesExpr)
	}

	if len(namespaceSelector.Include) > 0 {
		namespacesConditions := namespacesConditions(namespaceSelector.Include)
		includeNamespacesExpr := common.JoinWithAnd(
			// Ensure the k8s.namespace.name resource attribute is not nil,
			// so we don't drop logs without a namespace label
			common.ResourceAttributeIsNotNil(common.K8sNamespaceName),

			// Logs are dropped if the filter expression evaluates to true,
			// so we negate the match against included namespaces to keep only those
			common.Not(common.JoinWithOr(namespacesConditions...)),
		)
		filterExpressions = append(filterExpressions, includeNamespacesExpr)
	}

	return &FilterProcessor{
		Logs: FilterProcessorLogs{
			Log: filterExpressions,
		},
	}
}

func namespacesConditions(namespaces []string) []string {
	var conditions []string
	for _, ns := range namespaces {
		conditions = append(conditions, common.NamespaceEquals(ns))
	}

	return conditions
}

func dropIfInputSourceOTLPProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Logs: FilterProcessorLogs{
			Log: []string{
				// Drop all logs; the filter processor requires at least one valid condition expression,
				// to drop all logs, we use a condition that is always true for any log
				common.JoinWithOr(common.IsNotNil("log.observed_time"), common.IsNotNil("log.time")),
			},
		},
	}
}

func formatLogServicePipelineID(lp *telemetryv1alpha1.LogPipeline) string {
	return fmt.Sprintf("logs/%s", lp.Name)
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
	return common.FormatExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
}
