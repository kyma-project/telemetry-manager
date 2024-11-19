package metricpipeline

import (
	"context"
	"encoding/json"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type MetricPipelineDefaults struct {
	ExcludeNamespaces         []string
	RuntimeInputResources     RuntimeInputResourceDefaults
	DefaultOTLPOutputProtocol string
}

type RuntimeInputResourceDefaults struct {
	Pod         bool
	Container   bool
	Node        bool
	Volume      bool
	DaemonSet   bool
	Deployment  bool
	StatefulSet bool
	Job         bool
}

type DefaultingWebhookHandler struct {
	defaults MetricPipelineDefaults
	decoder  admission.Decoder
}

func NewDefaultingWebhookHandler(scheme *runtime.Scheme) *DefaultingWebhookHandler {
	return &DefaultingWebhookHandler{
		defaults: MetricPipelineDefaults{
			ExcludeNamespaces: []string{"kyma-system", "kube-system", "istio-system", "compass-system"},
			RuntimeInputResources: RuntimeInputResourceDefaults{
				Pod:         true,
				Container:   true,
				Node:        true,
				Volume:      true,
				DaemonSet:   true,
				Deployment:  true,
				StatefulSet: true,
				Job:         true,
			},
			DefaultOTLPOutputProtocol: telemetryv1alpha1.OTLPProtocolGRPC,
		},
		decoder: admission.NewDecoder(scheme),
	}
}

func (dh DefaultingWebhookHandler) Handle(ctx context.Context, request admission.Request) admission.Response {
	pipeline := &telemetryv1alpha1.MetricPipeline{}

	err := dh.decoder.Decode(request, pipeline)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	dh.applyDefaults(pipeline)

	marshaledPipeline, err := json.Marshal(pipeline)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(request.Object.Raw, marshaledPipeline)
}

func (dh DefaultingWebhookHandler) applyDefaults(pipeline *telemetryv1alpha1.MetricPipeline) {
	if pipeline.Spec.Input.Prometheus != nil && pipeline.Spec.Input.Prometheus.Namespaces == nil {
		pipeline.Spec.Input.Prometheus.Namespaces = &telemetryv1alpha1.NamespaceSelector{
			Exclude: dh.defaults.ExcludeNamespaces,
		}
	}

	if pipeline.Spec.Input.Istio != nil && pipeline.Spec.Input.Istio.Namespaces == nil {
		pipeline.Spec.Input.Istio.Namespaces = &telemetryv1alpha1.NamespaceSelector{
			Exclude: dh.defaults.ExcludeNamespaces,
		}
	}

	if pipeline.Spec.Output.OTLP != nil && pipeline.Spec.Output.OTLP.Protocol == "" {
		pipeline.Spec.Output.OTLP.Protocol = dh.defaults.DefaultOTLPOutputProtocol
	}

	if pipeline.Spec.Input.Runtime != nil && pipeline.Spec.Input.Runtime.Namespaces == nil {
		pipeline.Spec.Input.Runtime.Namespaces = &telemetryv1alpha1.NamespaceSelector{
			Exclude: dh.defaults.ExcludeNamespaces,
		}
	}

	if pipeline.Spec.Input.Runtime != nil {
		dh.applyRuntimeInputResourceDefaults(pipeline)
	}
}

func (dh DefaultingWebhookHandler) applyRuntimeInputResourceDefaults(pipeline *telemetryv1alpha1.MetricPipeline) {
	if pipeline.Spec.Input.Runtime.Resources == nil {
		pipeline.Spec.Input.Runtime.Resources = &telemetryv1alpha1.MetricPipelineRuntimeInputResources{}
	}

	if pipeline.Spec.Input.Runtime.Resources.Pod == nil {
		pipeline.Spec.Input.Runtime.Resources.Pod = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
			Enabled: &dh.defaults.RuntimeInputResources.Pod,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.Container == nil {
		pipeline.Spec.Input.Runtime.Resources.Container = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
			Enabled: &dh.defaults.RuntimeInputResources.Container,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.Node == nil {
		pipeline.Spec.Input.Runtime.Resources.Node = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
			Enabled: &dh.defaults.RuntimeInputResources.Node,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.Volume == nil {
		pipeline.Spec.Input.Runtime.Resources.Volume = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
			Enabled: &dh.defaults.RuntimeInputResources.Volume,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.DaemonSet == nil {
		pipeline.Spec.Input.Runtime.Resources.DaemonSet = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
			Enabled: &dh.defaults.RuntimeInputResources.DaemonSet,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.Deployment == nil {
		pipeline.Spec.Input.Runtime.Resources.Deployment = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
			Enabled: &dh.defaults.RuntimeInputResources.Deployment,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.StatefulSet == nil {
		pipeline.Spec.Input.Runtime.Resources.StatefulSet = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
			Enabled: &dh.defaults.RuntimeInputResources.StatefulSet,
		}
	}

	if pipeline.Spec.Input.Runtime.Resources.Job == nil {
		pipeline.Spec.Input.Runtime.Resources.Job = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
			Enabled: &dh.defaults.RuntimeInputResources.Job,
		}
	}
}
