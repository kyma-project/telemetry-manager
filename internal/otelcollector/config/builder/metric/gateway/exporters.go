package gateway

import (
	"context"
	"fmt"
	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/otlpoutput"
	"golang.org/x/exp/maps"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
)

func makeExportersConfig(ctx context.Context, c client.Reader, pipelines []v1alpha1.MetricPipeline) (common.ExportersConfig, common.PipelinesConfig, otlpoutput.EnvVars, error) {
	allVars := make(otlpoutput.EnvVars)
	exportersConfig := make(common.ExportersConfig)
	pipelinesConfig := make(common.PipelinesConfig)

	queueSize := 256 / len(pipelines)

	for _, pipeline := range pipelines {
		if pipeline.DeletionTimestamp != nil {
			continue
		}

		output := pipeline.Spec.Output
		exporterConfig, envVars, err := otlpoutput.MakeExportersConfig(ctx, c, output.Otlp, pipeline.Name, queueSize)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to make exporter config: %v", err)
		}

		var outputAliases []string

		maps.Copy(exportersConfig, exporterConfig)
		outputAliases = append(outputAliases, maps.Keys(exporterConfig)...)
		sort.Strings(outputAliases)
		pipelineConfig := makePipelineConfig(outputAliases)
		pipelineName := fmt.Sprintf("metrics/%s", pipeline.Name)
		pipelinesConfig[pipelineName] = pipelineConfig

		maps.Copy(allVars, envVars)
	}

	return exportersConfig, pipelinesConfig, allVars, nil
}

func makePipelineConfig(outputAliases []string) common.PipelineConfig {
	return common.PipelineConfig{
		Receivers:  []string{"otlp"},
		Processors: []string{"memory_limiter", "k8sattributes", "resource", "batch"},
		Exporters:  outputAliases,
	}
}
