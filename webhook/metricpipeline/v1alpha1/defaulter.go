package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

// +kubebuilder:object:generate=false
var _ webhook.CustomDefaulter = &defaulter{}

type defaulter struct {
	ExcludeNamespaces         []string
	RuntimeInputResources     runtimeInputResourceDefaults
	DefaultOTLPOutputProtocol string
	DiagnosticMetricsEnabled  bool
	EnvoyMetricsEnabled       bool
}

type runtimeInputResourceDefaults struct {
	Pod         bool
	Container   bool
	Node        bool
	Volume      bool
	DaemonSet   bool
	Deployment  bool
	StatefulSet bool
	Job         bool
}

func (md defaulter) Default(ctx context.Context, obj runtime.Object) error {
	pipeline, ok := obj.(*telemetryv1alpha1.MetricPipeline)
	if !ok {
		return fmt.Errorf("expected an MetricPipeline object but got %T", obj)
	}

	md.applyDefaults(pipeline)

	return nil
}

func (md defaulter) applyDefaults(pipeline *telemetryv1alpha1.MetricPipeline) {
	if prometheusInputEnabled(&pipeline.Spec.Input) {
		if pipeline.Spec.Input.Prometheus.Namespaces == nil {
			pipeline.Spec.Input.Prometheus.Namespaces = &telemetryv1alpha1.NamespaceSelector{
				Exclude: md.ExcludeNamespaces,
			}
		}

		if pipeline.Spec.Input.Prometheus.DiagnosticMetrics == nil {
			pipeline.Spec.Input.Prometheus.DiagnosticMetrics = &telemetryv1alpha1.MetricPipelineIstioInputDiagnosticMetrics{
				Enabled: &md.DiagnosticMetricsEnabled,
			}
		}
	}

	if istioInputEnabled(&pipeline.Spec.Input) {
		if pipeline.Spec.Input.Istio.Namespaces == nil {
			pipeline.Spec.Input.Istio.Namespaces = &telemetryv1alpha1.NamespaceSelector{
				Exclude: md.ExcludeNamespaces,
			}
		}

		if pipeline.Spec.Input.Istio.DiagnosticMetrics == nil {
			pipeline.Spec.Input.Istio.DiagnosticMetrics = &telemetryv1alpha1.MetricPipelineIstioInputDiagnosticMetrics{
				Enabled: &md.DiagnosticMetricsEnabled,
			}
		}

		if pipeline.Spec.Input.Istio.EnvoyMetrics == nil {
			pipeline.Spec.Input.Istio.EnvoyMetrics = &telemetryv1alpha1.EnvoyMetrics{
				Enabled: &md.EnvoyMetricsEnabled,
			}
		}
	}

	if runtimeInputEnabled(&pipeline.Spec.Input) {
		md.applyRuntimeInputResourceDefaults(pipeline)

		if pipeline.Spec.Input.Runtime.Namespaces == nil {
			pipeline.Spec.Input.Runtime.Namespaces = &telemetryv1alpha1.NamespaceSelector{
				Exclude: md.ExcludeNamespaces,
			}
		}
	}

	if pipeline.Spec.Input.OTLP == nil {
		pipeline.Spec.Input.OTLP = &telemetryv1alpha1.OTLPInput{}
	}

	if pipeline.Spec.Input.OTLP.Namespaces == nil {
		pipeline.Spec.Input.OTLP.Namespaces = &telemetryv1alpha1.NamespaceSelector{}
	}

	if pipeline.Spec.Output.OTLP != nil && pipeline.Spec.Output.OTLP.Protocol == "" {
		pipeline.Spec.Output.OTLP.Protocol = md.DefaultOTLPOutputProtocol
	}
}

func (md defaulter) applyRuntimeInputResourceDefaults(pipeline *telemetryv1alpha1.MetricPipeline) {
	if pipeline.Spec.Input.Runtime.Resources == nil {
		pipeline.Spec.Input.Runtime.Resources = &telemetryv1alpha1.MetricPipelineRuntimeInputResources{}
	}

	if pipeline.Spec.Input.Runtime.Resources.Pod == nil {
		pipeline.Spec.Input.Runtime.Resources.Pod = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
			Enabled: &md.RuntimeInputResources.Pod,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.Container == nil {
		pipeline.Spec.Input.Runtime.Resources.Container = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
			Enabled: &md.RuntimeInputResources.Container,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.Node == nil {
		pipeline.Spec.Input.Runtime.Resources.Node = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
			Enabled: &md.RuntimeInputResources.Node,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.Volume == nil {
		pipeline.Spec.Input.Runtime.Resources.Volume = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
			Enabled: &md.RuntimeInputResources.Volume,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.DaemonSet == nil {
		pipeline.Spec.Input.Runtime.Resources.DaemonSet = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
			Enabled: &md.RuntimeInputResources.DaemonSet,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.Deployment == nil {
		pipeline.Spec.Input.Runtime.Resources.Deployment = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
			Enabled: &md.RuntimeInputResources.Deployment,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.StatefulSet == nil {
		pipeline.Spec.Input.Runtime.Resources.StatefulSet = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
			Enabled: &md.RuntimeInputResources.StatefulSet,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.Job == nil {
		pipeline.Spec.Input.Runtime.Resources.Job = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
			Enabled: &md.RuntimeInputResources.Job,
		}
	}
}

func prometheusInputEnabled(input *telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Prometheus != nil && input.Prometheus.Enabled != nil && *input.Prometheus.Enabled
}

func istioInputEnabled(input *telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Istio != nil && input.Istio.Enabled != nil && *input.Istio.Enabled
}

func runtimeInputEnabled(input *telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Runtime != nil && input.Runtime.Enabled != nil && *input.Runtime.Enabled
}
