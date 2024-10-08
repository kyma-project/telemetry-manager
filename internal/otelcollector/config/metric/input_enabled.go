package metric

import telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"

func IsPrometheusInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Prometheus != nil && input.Prometheus.Enabled
}

func IsRuntimeInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Runtime != nil && input.Runtime.Enabled
}

func IsIstioInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Istio != nil && input.Istio.Enabled
}

func IsOtlpInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Otlp == nil || !input.Otlp.Disabled
}

func IsPrometheusDiagnosticMetricsEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Prometheus.DiagnosticMetrics != nil && input.Prometheus.DiagnosticMetrics.Enabled
}

func IsIstioDiagnosticMetricsEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Istio.DiagnosticMetrics != nil && input.Istio.DiagnosticMetrics.Enabled
}

func IsRuntimePodMetricsEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	// Define first isRuntimePodMetricsDisabled to ensure that the runtime pod metrics will be enabled by default
	// in case any of the fields (Resources, Pod or Enabled) is nil
	isRuntimePodMetricsDisabled := input.Runtime.Resources != nil &&
		input.Runtime.Resources.Pod != nil &&
		input.Runtime.Resources.Pod.Enabled != nil &&
		!*input.Runtime.Resources.Pod.Enabled

	return !isRuntimePodMetricsDisabled
}

func IsRuntimeContainerMetricsEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	// Define first isRuntimeContainerMetricsDisabled to ensure that the runtime container metrics will be enabled by default
	// in case any of the fields (Resources, Pod or Enabled) is nil
	isRuntimeContainerMetricsDisabled := input.Runtime.Resources != nil &&
		input.Runtime.Resources.Container != nil &&
		input.Runtime.Resources.Container.Enabled != nil &&
		!*input.Runtime.Resources.Container.Enabled

	return !isRuntimeContainerMetricsDisabled
}

func IsRuntimeNodeMetricsEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	// Runtime node metrics are disabled by default
	// If any of the fields (Resources, Node or Enabled) is nil, the node metrics will be disabled
	return input.Runtime.Resources != nil &&
		input.Runtime.Resources.Node != nil &&
		input.Runtime.Resources.Node.Enabled != nil &&
		*input.Runtime.Resources.Node.Enabled
}

func IsRuntimeVolumeMetricsEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	// Runtime volume metrics are disabled by default
	// If any of the fields (Resources, Volume or Enabled) is nil, the volume metrics will be disabled
	return input.Runtime.Resources != nil &&
		input.Runtime.Resources.Volume != nil &&
		input.Runtime.Resources.Volume.Enabled != nil &&
		*input.Runtime.Resources.Volume.Enabled
}
