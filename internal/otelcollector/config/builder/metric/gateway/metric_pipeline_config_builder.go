package gateway

import (
	"context"
	"fmt"
	"sort"

	"golang.org/x/exp/maps"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/otlpoutput"
)

type otlpOutputConfigBuilder struct {
	ctx       context.Context
	c         client.Reader
	pipeline  telemetryv1alpha1.MetricPipeline
	queueSize int
}

func (b *otlpOutputConfigBuilder) build() (map[string]common.BaseGatewayExporterConfig, otlpoutput.EnvVars, error) {
	return otlpoutput.MakeExporterConfigs(b.ctx, b.c, b.pipeline.Spec.Output.Otlp, b.pipeline.Name, b.queueSize)
}

// addComponentsForMetricPipeline enriches a Config with components for a given MetricPipeline (exporters, processors, etc.). It also adds needed env vars to a map
func addComponentsForMetricPipeline(otlpBuilder otlpOutputConfigBuilder, pipeline telemetryv1alpha1.MetricPipeline, config *Config, envVars otlpoutput.EnvVars) error {
	if pipeline.DeletionTimestamp != nil {
		return nil
	}

	exporterConfigs, pipelineEnvVars, err := otlpBuilder.build()
	if err != nil {
		return fmt.Errorf("failed to make exporter config: %w", err)
	}

	maps.Copy(envVars, pipelineEnvVars)

	var exporterIDs []string
	for exporterID, exporterConfig := range exporterConfigs {
		config.Exporters[exporterID] = ExporterConfig{BaseGatewayExporterConfig: exporterConfig}
		exporterIDs = append(exporterIDs, exporterID)
	}

	config.Service.Pipelines[fmt.Sprintf("metrics/%s", pipeline.Name)] = makePipelineConfig(exporterIDs)

	return nil
}

func makePipelineConfig(exporterIDs []string) common.PipelineConfig {
	sort.Strings(exporterIDs)

	return common.PipelineConfig{
		Receivers:  []string{"otlp"},
		Processors: []string{"memory_limiter", "k8sattributes", "resource", "batch"},
		Exporters:  exporterIDs,
	}
}
