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

type buildComponentFunc = common.BuildComponentFunc[*telemetryv1alpha1.LogPipeline]

type Builder struct {
	common.ComponentBuilder[*telemetryv1alpha1.LogPipeline]

	Reader client.Reader
}

type BuildOptions struct {
	ClusterName   string
	ClusterUID    string
	CloudProvider string
	Enrichments   *operatorv1alpha1.EnrichmentSpec
	ModuleVersion string
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.LogPipeline, opts BuildOptions) (*common.Config, common.EnvVars, error) {
	b.Config = common.NewConfig()
	b.EnvVars = make(common.EnvVars)

	// Iterate over each LogPipeline CR and enrich the config with pipeline-specific components
	queueSize := common.BatchingMaxQueueSize / len(pipelines)

	for _, pipeline := range pipelines {
		pipelineID := formatLogServicePipelineID(&pipeline)
		if err := b.AddServicePipeline(ctx, &pipeline, pipelineID,
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
			b.addIstioAccessLogsEnrichmentProcessor(opts),
			b.addUserDefinedTransformProcessor(),
			b.addUserDefinedFilterProcessor(),
			b.addBatchProcessor(),
			b.addOTLPExporter(queueSize),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add service pipeline: %w", err)
		}
	}

	return b.Config, b.EnvVars, nil
}

func (b *Builder) addOTLPReceiver() buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDOTLPReceiver),
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
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDMemoryLimiterProcessor),
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
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDSetObservedTimeIfZeroProcessor),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return common.LogTransformProcessorConfig([]common.TransformProcessorStatements{{
				Conditions: []string{"log.observed_time_unix_nano == 0"},
				Statements: []string{"set(log.observed_time, Now())"},
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

func (b *Builder) addIstioNoiseFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDIstioNoiseFilterProcessor),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return &common.IstioNoiseFilterProcessor{}
		},
	)
}

func (b *Builder) addDropIfInputSourceOTLPProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropIfInputSourceOTLPProcessor),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			if logpipelineutils.IsOTLPInputEnabled(lp.Spec.Input) {
				return nil // Skip this processor if OTLP input is enabled
			}

			return dropIfInputSourceOTLPProcessorConfig()
		},
	)
}

func (b *Builder) addNamespaceFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
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

func (b *Builder) addIstioAccessLogsEnrichmentProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDIstioEnrichmentProcessor),
		func(lp *telemetryv1alpha1.LogPipeline) any {
			return &IstioEnrichmentProcessor{
				ScopeVersion: opts.ModuleVersion,
			}
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

func (b *Builder) addUserDefinedFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		formatUserDefinedFilterProcessorID,
		func(lp *telemetryv1alpha1.LogPipeline) any {
			if len(lp.Spec.Filter.Conditions) == 0 {
				return nil // No Filter, no processor need
			}

			filterStatements := common.FilterSpecsToProcessorStatements(lp.Spec.Filter)
			filterProcessor := common.LogFilterProcessorConfig(filterStatements)

			return filterProcessor
		},
	)
}

//nolint:mnd // hardcoded values
func (b *Builder) addBatchProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDBatchProcessor),
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
	return b.AddExporter(
		formatOTLPExporterID,
		func(ctx context.Context, lp *telemetryv1alpha1.LogPipeline) (any, common.EnvVars, error) {
			otlpExporterBuilder := common.NewOTLPExporterConfigBuilder(
				b.Reader,
				lp.Spec.Output.OTLP,
				lp.Name,
				queueSize,
				common.SignalTypeLog,
			)

			return otlpExporterBuilder.OTLPExporterConfig(ctx)
		},
	)
}

// Helper functions

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

func shouldFilterByNamespace(namespaceSelector *telemetryv1alpha1.NamespaceSelector) bool {
	return namespaceSelector != nil && (len(namespaceSelector.Include) > 0 || len(namespaceSelector.Exclude) > 0)
}

func formatLogServicePipelineID(lp *telemetryv1alpha1.LogPipeline) string {
	return fmt.Sprintf("logs/%s", lp.Name)
}

func formatNamespaceFilterID(lp *telemetryv1alpha1.LogPipeline) string {
	return fmt.Sprintf(common.ComponentIDNamespaceFilterProcessor, lp.Name)
}

func formatUserDefinedTransformProcessorID(lp *telemetryv1alpha1.LogPipeline) string {
	return fmt.Sprintf(common.ComponentIDUserDefinedTransformProcessor, lp.Name)
}

func formatUserDefinedFilterProcessorID(lp *telemetryv1alpha1.LogPipeline) string {
	return fmt.Sprintf(common.ComponentIDUserDefinedFilterProcessor, lp.Name)
}

func formatOTLPExporterID(pipeline *telemetryv1alpha1.LogPipeline) string {
	return common.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
}
