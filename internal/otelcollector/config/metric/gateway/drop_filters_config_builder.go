package gateway

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	metricpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/metricpipeline"
)

type pipelineDropFilterConfig struct {
	requireRuntimeInputFilter    bool
	requireIstioInputFilter      bool
	requirePrometheusInputFilter bool
	requireOTLPInputFilter       bool
	requireEnvoyMetricsFilter    bool
}

func buildPipelinesDropFilterConfigs(pipelines []telemetryv1alpha1.MetricPipeline) map[string]pipelineDropFilterConfig {
	configs := make(map[string]pipelineDropFilterConfig)
	for _, pipeline := range pipelines {
		configs[pipeline.Name] = pipelineDropFilterConfig{
			requireRuntimeInputFilter:    !metricpipelineutils.IsRuntimeInputEnabled(pipeline.Spec.Input) && requireDropRuntimeInputFilter(pipeline, pipelines),
			requireIstioInputFilter:      !metricpipelineutils.IsIstioInputEnabled(pipeline.Spec.Input) && requireDropIstioInputFilter(pipeline, pipelines),
			requirePrometheusInputFilter: !metricpipelineutils.IsPrometheusInputEnabled(pipeline.Spec.Input) && requireDropPrometheusInputFilter(pipeline, pipelines),
			requireOTLPInputFilter:       !metricpipelineutils.IsOTLPInputEnabled(pipeline.Spec.Input) && hasOtherPipelineWithOTLPInput(pipeline, pipelines),
			requireEnvoyMetricsFilter:    (!metricpipelineutils.IsIstioInputEnabled(pipeline.Spec.Input) || !metricpipelineutils.IsEnvoyMetricsEnabled(pipeline.Spec.Input)) && requireDropEnvoyMetricsFilter(pipeline, pipelines),
		}
	}

	return configs
}

func requireDropRuntimeInputFilter(pipeline telemetryv1alpha1.MetricPipeline, pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for _, other := range pipelines {
		if other.Name != pipeline.Name && metricpipelineutils.IsRuntimeInputEnabled(other.Spec.Input) {
			return true
		}
	}

	return false
}

func requireDropIstioInputFilter(pipeline telemetryv1alpha1.MetricPipeline, pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for _, other := range pipelines {
		if other.Name != pipeline.Name && metricpipelineutils.IsIstioInputEnabled(other.Spec.Input) {
			return true
		}
	}

	return false
}

func requireDropPrometheusInputFilter(pipeline telemetryv1alpha1.MetricPipeline, pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for _, other := range pipelines {
		if other.Name != pipeline.Name && metricpipelineutils.IsPrometheusInputEnabled(other.Spec.Input) {
			return true
		}
	}

	return false
}

func hasOtherPipelineWithOTLPInput(pipeline telemetryv1alpha1.MetricPipeline, pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for _, other := range pipelines {
		if other.Name != pipeline.Name && metricpipelineutils.IsOTLPInputEnabled(other.Spec.Input) {
			return true
		}
	}

	return false
}

func requireDropEnvoyMetricsFilter(pipeline telemetryv1alpha1.MetricPipeline, pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for _, other := range pipelines {
		if other.Name != pipeline.Name && metricpipelineutils.IsIstioInputEnabled(other.Spec.Input) && metricpipelineutils.IsEnvoyMetricsEnabled(other.Spec.Input) {
			return true
		}
	}

	return false
}
