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

func IsOTLPInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Otlp == nil || !input.Otlp.Disabled
}

func IsPrometheusDiagnosticInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Prometheus.DiagnosticMetrics != nil && input.Prometheus.DiagnosticMetrics.Enabled
}

func IsIstioDiagnosticInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Istio.DiagnosticMetrics != nil && input.Istio.DiagnosticMetrics.Enabled
}

func IsRuntimePodInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	// Runtime pod metrics should be enabled by default if any of the fields (Resources, Pod or Enabled) is nil
	if input.Runtime.Resources == nil || input.Runtime.Resources.Pod == nil || input.Runtime.Resources.Pod.Enabled == nil {
		return true
	}
	return *input.Runtime.Resources.Pod.Enabled
}

func IsRuntimeContainerInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	// Runtime container metrics should be enabled by default if any of the fields (Resources, Container or Enabled) is nil
	if input.Runtime.Resources == nil || input.Runtime.Resources.Container == nil || input.Runtime.Resources.Container.Enabled == nil {
		return true
	}
	return *input.Runtime.Resources.Container.Enabled
}

func IsRuntimeNodeInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	// Runtime node metrics should be disabled by default if any of the fields (Resources, Node or Enabled) is nil
	if input.Runtime.Resources == nil || input.Runtime.Resources.Node == nil || input.Runtime.Resources.Node.Enabled == nil {
		return false
	}
	return *input.Runtime.Resources.Node.Enabled
}

func IsRuntimeVolumeInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	// Runtime volume metrics should be disabled by default if any of the fields (Resources, Volume or Enabled) is nil
	if input.Runtime.Resources == nil || input.Runtime.Resources.Volume == nil || input.Runtime.Resources.Volume.Enabled == nil {
		return false
	}
	return *input.Runtime.Resources.Volume.Enabled
}
