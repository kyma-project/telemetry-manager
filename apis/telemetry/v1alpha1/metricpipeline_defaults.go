package v1alpha1

import "k8s.io/utils/pointer"

func SetMetricPipelineDefaults(pipeline *MetricPipeline) {
	input := pipeline.Spec.Input
	if input.Prometheus.Namespaces.System == nil {
		pipeline.Spec.Input.Prometheus.Namespaces.System = pointer.Bool(false)
	}
	if input.Runtime.Namespaces.System == nil {
		pipeline.Spec.Input.Runtime.Namespaces.System = pointer.Bool(false)
	}
	if input.Istio.Namespaces.System == nil {
		pipeline.Spec.Input.Istio.Namespaces.System = pointer.Bool(true)
	}
	if input.Otlp.Namespaces.System == nil {
		pipeline.Spec.Input.Otlp.Namespaces.System = pointer.Bool(false)
	}
}
