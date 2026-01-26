package v1beta1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	metricpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/metricpipeline"
)

// +kubebuilder:object:generate=false
var _ webhook.CustomDefaulter = &defaulter{}

type defaulter struct {
	ExcludeNamespaces         []string
	OTLPInputEnabled          bool
	RuntimeInputResources     runtimeInputResourceDefaults
	DefaultOTLPOutputProtocol telemetryv1beta1.OTLPProtocol
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
	pipeline, ok := obj.(*telemetryv1beta1.MetricPipeline)
	if !ok {
		return fmt.Errorf("expected an MetricPipeline object but got %T", obj)
	}

	md.applyDefaults(pipeline)

	return nil
}

func (md defaulter) applyDefaults(pipeline *telemetryv1beta1.MetricPipeline) {
	if metricpipelineutils.IsPrometheusInputEnabled(pipeline.Spec.Input) {
		if pipeline.Spec.Input.Prometheus.Namespaces == nil {
			pipeline.Spec.Input.Prometheus.Namespaces = &telemetryv1beta1.NamespaceSelector{
				Exclude: md.ExcludeNamespaces,
			}
		}

		if pipeline.Spec.Input.Prometheus.DiagnosticMetrics == nil {
			pipeline.Spec.Input.Prometheus.DiagnosticMetrics = &telemetryv1beta1.MetricPipelineIstioInputDiagnosticMetrics{
				Enabled: &md.DiagnosticMetricsEnabled,
			}
		}
	}

	if metricpipelineutils.IsIstioInputEnabled(pipeline.Spec.Input) {
		if pipeline.Spec.Input.Istio.Namespaces == nil {
			pipeline.Spec.Input.Istio.Namespaces = &telemetryv1beta1.NamespaceSelector{
				Exclude: md.ExcludeNamespaces,
			}
		}

		if pipeline.Spec.Input.Istio.EnvoyMetrics == nil {
			pipeline.Spec.Input.Istio.EnvoyMetrics = &telemetryv1beta1.EnvoyMetrics{
				Enabled: &md.EnvoyMetricsEnabled,
			}
		}

		if pipeline.Spec.Input.Istio.DiagnosticMetrics == nil {
			pipeline.Spec.Input.Istio.DiagnosticMetrics = &telemetryv1beta1.MetricPipelineIstioInputDiagnosticMetrics{
				Enabled: &md.DiagnosticMetricsEnabled,
			}
		}
	}

	if metricpipelineutils.IsRuntimeInputEnabled(pipeline.Spec.Input) {
		md.applyRuntimeInputResourceDefaults(pipeline)

		if pipeline.Spec.Input.Runtime.Namespaces == nil {
			pipeline.Spec.Input.Runtime.Namespaces = &telemetryv1beta1.NamespaceSelector{
				Exclude: md.ExcludeNamespaces,
			}
		}
	}

	if pipeline.Spec.Input.OTLP == nil {
		pipeline.Spec.Input.OTLP = &telemetryv1beta1.OTLPInput{}
	}

	if pipeline.Spec.Input.OTLP.Enabled == nil {
		pipeline.Spec.Input.OTLP.Enabled = &md.OTLPInputEnabled
	}

	if ptr.Deref(pipeline.Spec.Input.OTLP.Enabled, false) && pipeline.Spec.Input.OTLP.Namespaces == nil {
		pipeline.Spec.Input.OTLP.Namespaces = &telemetryv1beta1.NamespaceSelector{}
	}

	if pipeline.Spec.Output.OTLP != nil && pipeline.Spec.Output.OTLP.Protocol == "" {
		pipeline.Spec.Output.OTLP.Protocol = md.DefaultOTLPOutputProtocol
	}
}

func (md defaulter) applyRuntimeInputResourceDefaults(pipeline *telemetryv1beta1.MetricPipeline) {
	if pipeline.Spec.Input.Runtime.Resources == nil {
		pipeline.Spec.Input.Runtime.Resources = &telemetryv1beta1.MetricPipelineRuntimeInputResources{}
	}

	if pipeline.Spec.Input.Runtime.Resources.Pod == nil {
		pipeline.Spec.Input.Runtime.Resources.Pod = &telemetryv1beta1.MetricPipelineRuntimeInputResource{
			Enabled: &md.RuntimeInputResources.Pod,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.Container == nil {
		pipeline.Spec.Input.Runtime.Resources.Container = &telemetryv1beta1.MetricPipelineRuntimeInputResource{
			Enabled: &md.RuntimeInputResources.Container,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.Node == nil {
		pipeline.Spec.Input.Runtime.Resources.Node = &telemetryv1beta1.MetricPipelineRuntimeInputResource{
			Enabled: &md.RuntimeInputResources.Node,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.Volume == nil {
		pipeline.Spec.Input.Runtime.Resources.Volume = &telemetryv1beta1.MetricPipelineRuntimeInputResource{
			Enabled: &md.RuntimeInputResources.Volume,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.DaemonSet == nil {
		pipeline.Spec.Input.Runtime.Resources.DaemonSet = &telemetryv1beta1.MetricPipelineRuntimeInputResource{
			Enabled: &md.RuntimeInputResources.DaemonSet,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.Deployment == nil {
		pipeline.Spec.Input.Runtime.Resources.Deployment = &telemetryv1beta1.MetricPipelineRuntimeInputResource{
			Enabled: &md.RuntimeInputResources.Deployment,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.StatefulSet == nil {
		pipeline.Spec.Input.Runtime.Resources.StatefulSet = &telemetryv1beta1.MetricPipelineRuntimeInputResource{
			Enabled: &md.RuntimeInputResources.StatefulSet,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.Job == nil {
		pipeline.Spec.Input.Runtime.Resources.Job = &telemetryv1beta1.MetricPipelineRuntimeInputResource{
			Enabled: &md.RuntimeInputResources.Job,
		}
	}
}
