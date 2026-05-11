package otlpgateway

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
)

type buildTraceComponentFunc = common.BuildComponentFunc[*telemetryv1beta1.TracePipeline]
type buildLogComponentFunc = common.BuildComponentFunc[*telemetryv1beta1.LogPipeline]
type buildMetricComponentFunc = common.BuildComponentFunc[*telemetryv1beta1.MetricPipeline]

// Builder builds OTel Collector configuration for the OTLP Gateway.
// Handles TracePipelines, LogPipelines, and MetricPipelines.
type Builder struct {
	Reader client.Reader
}

// BuildOptions contains options for building the OTLP Gateway configuration.
type BuildOptions struct {
	LogPipelines    []telemetryv1beta1.LogPipeline
	TracePipelines  []telemetryv1beta1.TracePipeline
	MetricPipelines []telemetryv1beta1.MetricPipeline

	Cluster     common.ClusterOptions
	Enrichments *operatorv1beta1.EnrichmentSpec
	// ServiceEnrichment specifies the service enrichment strategy (temporary)
	ServiceEnrichment string
	// ModuleVersion is needed for Istio enrichment in log pipelines and instrumentation scope in metric pipelines
	ModuleVersion string
	// GatewayNamespace is needed for the K8sLeaderElector extension used by the KymaStats receiver in metric pipelines
	GatewayNamespace string
	// VpaActive indicates whether VPA is active (VPA CRD exists and VPA is enabled via annotation in Telemetry CR).
	VpaActive bool
}

// Build creates OTel Collector configuration from TracePipeline, LogPipeline, and MetricPipeline CRs.
func (b *Builder) Build(ctx context.Context, opts BuildOptions) (*common.Config, common.EnvVars, error) {
	b.sortPipelinesByName(&opts)

	config := common.NewConfig()
	envVars := make(common.EnvVars)

	if opts.VpaActive {
		config.DisableGoMemLimit()
	}

	// Build trace pipelines
	traceBuilder := common.ComponentBuilder[*telemetryv1beta1.TracePipeline]{
		Config:  config,
		EnvVars: envVars,
	}
	if err := b.buildTracePipelines(ctx, &traceBuilder, opts); err != nil {
		return nil, nil, err
	}

	// Build log pipelines
	logBuilder := common.ComponentBuilder[*telemetryv1beta1.LogPipeline]{
		Config:  config,
		EnvVars: envVars,
	}
	if err := b.buildLogPipelines(ctx, &logBuilder, opts); err != nil {
		return nil, nil, err
	}

	// Build metric pipelines
	metricBuilder := common.ComponentBuilder[*telemetryv1beta1.MetricPipeline]{
		Config:  config,
		EnvVars: envVars,
	}
	if err := b.buildMetricPipelines(ctx, &metricBuilder, opts); err != nil {
		return nil, nil, err
	}

	return config, envVars, nil
}

// sortPipelinesByName sorts pipelines by name to ensure consistent order and checksum for generated ConfigMap
func (b *Builder) sortPipelinesByName(opts *BuildOptions) {
	slices.SortFunc(opts.LogPipelines, func(a, b telemetryv1beta1.LogPipeline) int {
		return strings.Compare(a.Name, b.Name)
	})
	slices.SortFunc(opts.TracePipelines, func(a, b telemetryv1beta1.TracePipeline) int {
		return strings.Compare(a.Name, b.Name)
	})
	slices.SortFunc(opts.MetricPipelines, func(a, b telemetryv1beta1.MetricPipeline) int {
		return strings.Compare(a.Name, b.Name)
	})
}

// ================================================================================
// SHARED COMPONENT CONFIG BUILDERS
// ================================================================================

// otlpReceiverConfig returns the shared OTLP receiver configuration used by all signal types.
//
//nolint:mnd // port numbers are defined in the ports package
func otlpReceiverConfig() *common.OTLPReceiverConfig {
	return &common.OTLPReceiverConfig{
		Protocols: common.ReceiverProtocols{
			HTTP: common.Endpoint{Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OTLPHTTP)},
			GRPC: common.Endpoint{Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OTLPGRPC)},
		},
	}
}

// memoryLimiterConfig returns the shared memory limiter configuration used by all signal types.
//
//nolint:mnd // hardcoded memory limiter values
func memoryLimiterConfig() *common.MemoryLimiterConfig {
	return &common.MemoryLimiterConfig{
		CheckInterval:        "1s",
		LimitPercentage:      75,
		SpikeLimitPercentage: 15,
	}
}

// k8sAttributesProcessorConfig returns the shared K8s attributes processor configuration.
func k8sAttributesProcessorConfig(opts BuildOptions) any {
	useOTelServiceEnrichment := opts.ServiceEnrichment == commonresources.AnnotationValueTelemetryServiceEnrichmentOtel
	return common.K8sAttributesProcessor(opts.Enrichments, useOTelServiceEnrichment)
}

// serviceEnrichmentProcessorConfig returns the shared service enrichment processor configuration.
func serviceEnrichmentProcessorConfig(opts BuildOptions) any {
	if opts.ServiceEnrichment == commonresources.AnnotationValueTelemetryServiceEnrichmentOtel {
		return nil
	}

	return common.ResolveServiceName()
}

// istioNoiseFilterProcessorConfig returns the shared Istio noise filter processor configuration.
func istioNoiseFilterProcessorConfig() *common.IstioNoiseFilterProcessorConfig {
	return &common.IstioNoiseFilterProcessorConfig{}
}
