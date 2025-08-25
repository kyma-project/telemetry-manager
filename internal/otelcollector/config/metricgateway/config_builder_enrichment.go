package metricgateway

import (
	"context"
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
)

func (b *Builder) addEnrichmentServicePipeline(ctx context.Context, mp *telemetryv1alpha1.MetricPipeline, fs ...buildComponentFunc) error {
	// Add an empty pipeline to the config
	pipelineID := formatEnrichmentServicePipelineID(mp)
	b.config.Service.Pipelines[pipelineID] = common.Pipeline{}

	for _, f := range fs {
		if err := f(ctx, mp); err != nil {
			return fmt.Errorf("failed to add component: %w", err)
		}
	}

	return nil
}

func (b *Builder) addEnrichmentReceiver(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return common.AddReceiver(b.config, componentIDFunc, configFunc, formatEnrichmentServicePipelineID)
}

func (b *Builder) addEnrichmentProcessor(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return common.AddProcessor(b.config, componentIDFunc, configFunc, formatEnrichmentServicePipelineID)
}

func (b *Builder) addEnrichmentExporter(componentIDFunc componentIDFunc, configFunc exporterComponentConfigFunc) buildComponentFunc {
	return common.AddExporter(b.config, b.envVars, componentIDFunc, configFunc, formatEnrichmentServicePipelineID)
}

func (b *Builder) addRoutingConnectorAsReceiver() buildComponentFunc {
	return b.addEnrichmentReceiver(
		staticComponentID(common.ComponentIDRoutingConnector),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return enrichmentRoutingConnectorConfig(mp)
		},
	)
}

func (b *Builder) addK8sAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.addEnrichmentProcessor(
		staticComponentID(common.ComponentIDK8sAttributesProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.K8sAttributesProcessorConfig(opts.Enrichments)
		},
	)
}

func (b *Builder) addServiceEnrichmentProcessor() buildComponentFunc {
	return b.addEnrichmentProcessor(
		staticComponentID(common.ComponentIDServiceEnrichmentProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.ResolveServiceNameConfig()
		},
	)
}

func (b *Builder) addForwardConnectorAsExporter() buildComponentFunc {
	return b.addEnrichmentExporter(
		staticComponentID(common.ComponentIDForwardConnector),
		func(ctx context.Context, mp *telemetryv1alpha1.MetricPipeline) (any, common.EnvVars, error) {
			return &common.ForwardConnector{}, nil, nil
		},
	)
}
