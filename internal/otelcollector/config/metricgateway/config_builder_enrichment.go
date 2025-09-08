package metricgateway

import (
	"context"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
)

func (b *Builder) addEnrichmentRoutingReceiver() buildComponentFunc {
	return b.AddReceiver(
		formatRoutingConnectorID,
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.SkipEnrichmentRoutingConnectorConfig(
				[]string{formatEnrichmentServicePipelineID(mp)},
				[]string{formatOutputServicePipelineID(mp)},
			)
		},
	)
}

func (b *Builder) addK8sAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDK8sAttributesProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.K8sAttributesProcessorConfig(opts.Enrichments)
		},
	)
}

func (b *Builder) addServiceEnrichmentProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDServiceEnrichmentProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.ResolveServiceNameConfig()
		},
	)
}

func (b *Builder) addEnrichmentForwardExporter() buildComponentFunc {
	return b.AddExporter(
		formatForwardConnectorID,
		func(ctx context.Context, mp *telemetryv1alpha1.MetricPipeline) (any, common.EnvVars, error) {
			return &common.ForwardConnector{}, nil, nil
		},
	)
}