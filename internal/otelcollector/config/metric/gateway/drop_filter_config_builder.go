package gateway

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/utils/metricpipeline"
)

type pipelineDropFilterConfig struct {
	requireRuntimeInputFilter    bool
	requireIstioInputFilter      bool
	requirePrometheusInputFilter bool
	requireOTLPInputFilter       bool
	requireEnvoyMetricsFilter    bool
}

func getPipelineDropFilterConfigs(pipelines []telemetryv1alpha1.MetricPipeline) map[string]pipelineDropFilterConfig {
	configs := make(map[string]pipelineDropFilterConfig)
	for _, pipeline := range pipelines {
		configs[pipeline.Name] = pipelineDropFilterConfig{
			requireRuntimeInputFilter:    !metricpipeline.IsRuntimeInputEnabled(pipeline.Spec.Input) && hasOtherPipelineWithRuntimeInput(pipeline, pipelines),
			requireIstioInputFilter:      !metricpipeline.IsIstioInputEnabled(pipeline.Spec.Input) && hasOtherPipelineWithIstioInput(pipeline, pipelines),
			requirePrometheusInputFilter: !metricpipeline.IsPrometheusInputEnabled(pipeline.Spec.Input) && hasOtherPipelineWithPrometheusInput(pipeline, pipelines),
			requireOTLPInputFilter:       !metricpipeline.IsOTLPInputEnabled(pipeline.Spec.Input) && hasOtherPipelineWithOTLPInput(pipeline, pipelines),
			requireEnvoyMetricsFilter:    (!metricpipeline.IsIstioInputEnabled(pipeline.Spec.Input) || !metricpipeline.IsEnvoyMetricsEnabled(pipeline.Spec.Input)) && hasOtherPipelineWithEnvoyMetrics(pipeline, pipelines),
		}
	}

	return configs
}

func hasOtherPipelineWithRuntimeInput(pipeline telemetryv1alpha1.MetricPipeline, pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for _, other := range pipelines {
		if other.Name != pipeline.Name && metricpipeline.IsRuntimeInputEnabled(other.Spec.Input) {
			return true
		}
	}

	return false
}

func hasOtherPipelineWithIstioInput(pipeline telemetryv1alpha1.MetricPipeline, pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for _, other := range pipelines {
		if other.Name != pipeline.Name && metricpipeline.IsIstioInputEnabled(other.Spec.Input) {
			return true
		}
	}

	return false
}

func hasOtherPipelineWithPrometheusInput(pipeline telemetryv1alpha1.MetricPipeline, pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for _, other := range pipelines {
		if other.Name != pipeline.Name && metricpipeline.IsPrometheusInputEnabled(other.Spec.Input) {
			return true
		}
	}

	return false
}

func hasOtherPipelineWithOTLPInput(pipeline telemetryv1alpha1.MetricPipeline, pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for _, other := range pipelines {
		if other.Name != pipeline.Name && metricpipeline.IsOTLPInputEnabled(other.Spec.Input) {
			return true
		}
	}

	return false
}

func hasOtherPipelineWithEnvoyMetrics(pipeline telemetryv1alpha1.MetricPipeline, pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for _, other := range pipelines {
		if other.Name != pipeline.Name && metricpipeline.IsIstioInputEnabled(other.Spec.Input) && metricpipeline.IsEnvoyMetricsEnabled(other.Spec.Input) {
			return true
		}
	}

	return false
}
