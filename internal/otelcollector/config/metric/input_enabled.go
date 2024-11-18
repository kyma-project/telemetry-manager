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
	return input.OTLP == nil || !input.OTLP.Disabled
}

func IsPrometheusDiagnosticInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Prometheus.DiagnosticMetrics != nil && input.Prometheus.DiagnosticMetrics.Enabled
}

func IsIstioDiagnosticInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Istio.DiagnosticMetrics != nil && input.Istio.DiagnosticMetrics.Enabled
}

func IsRuntimePodInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Runtime.Resources != nil && input.Runtime.Resources.Pod != nil && input.Runtime.Resources.Pod.Enabled
}

func IsRuntimeContainerInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Runtime.Resources != nil && input.Runtime.Resources.Container != nil && input.Runtime.Resources.Container.Enabled
}

func IsRuntimeNodeInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Runtime.Resources != nil && input.Runtime.Resources.Node != nil && input.Runtime.Resources.Node.Enabled
}

func IsRuntimeVolumeInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Runtime.Resources != nil && input.Runtime.Resources.Volume != nil && input.Runtime.Resources.Volume.Enabled
}

func IsRuntimeStatefulSetInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Runtime.Resources != nil && input.Runtime.Resources.StatefulSet != nil && input.Runtime.Resources.StatefulSet.Enabled
}

func IsRuntimeDeploymentInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Runtime.Resources != nil && input.Runtime.Resources.Deployment != nil && input.Runtime.Resources.Deployment.Enabled
}

func IsRuntimeDaemonSetInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Runtime.Resources != nil && input.Runtime.Resources.DaemonSet != nil && input.Runtime.Resources.DaemonSet.Enabled
}

func IsRuntimeJobInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Runtime.Resources != nil && input.Runtime.Resources.Job != nil && input.Runtime.Resources.Job.Enabled
}
