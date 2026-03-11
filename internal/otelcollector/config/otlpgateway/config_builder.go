package otlpgateway

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
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
}

// Build creates OTel Collector configuration from TracePipeline, LogPipeline, and MetricPipeline CRs.
func (b *Builder) Build(ctx context.Context, opts BuildOptions) (*common.Config, common.EnvVars, error) {
	config := common.NewConfig()
	envVars := make(common.EnvVars)

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
