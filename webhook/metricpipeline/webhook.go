/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metricpipeline

import (
	"context"
	"fmt"
	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

type MetricPipelineDefaulter struct {
	ExcludeNamespaces         []string
	RuntimeInputResources     RuntimeInputResources
	DefaultOTLPOutputProtocol v1beta1.OTLPProtocol
}

type RuntimeInputResources struct {
	Pod         bool
	Container   bool
	Node        bool
	Volume      bool
	DaemonSet   bool
	Deployment  bool
	StatefulSet bool
	Job         bool
}

func SetupMetricPipelineWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		WithDefaulter(&MetricPipelineDefaulter{
			ExcludeNamespaces: []string{"kyma-system", "kube-system", "istio-system", "compass-system"},
			RuntimeInputResources: RuntimeInputResources{
				Pod:         true,
				Container:   true,
				Node:        false,
				Volume:      false,
				DaemonSet:   false,
				Deployment:  false,
				StatefulSet: false,
				Job:         false,
			},
			DefaultOTLPOutputProtocol: v1beta1.OTLPProtocolGRPC,
		}).
		For(&v1beta1.MetricPipeline{}).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-metricpipeline,mutating=true,failurePolicy=fail,sideEffects=None,groups=telemetry.kyma-project.io,resources=metricpipelines,verbs=create;update,versions=v1beta1,name=mmetricpipeline.kb.io,admissionReviewVersions=v1

var _ webhook.CustomDefaulter = &MetricPipelineDefaulter{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (md MetricPipelineDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	pipeline, ok := obj.(*v1beta1.MetricPipeline)

	if !ok {
		return fmt.Errorf("expected a MetricPipeline but got a %T", obj)
	}

	md.applyDefaults(pipeline)

	return nil
}

func (md MetricPipelineDefaulter) applyDefaults(pipeline *v1beta1.MetricPipeline) {
	if pipeline.Spec.Input.Prometheus != nil && pipeline.Spec.Input.Prometheus.Namespaces == nil {
		pipeline.Spec.Input.Prometheus.Namespaces = &v1beta1.NamespaceSelector{
			Exclude: md.ExcludeNamespaces,
		}
	}

	if pipeline.Spec.Input.Runtime != nil && pipeline.Spec.Input.Runtime.Namespaces == nil {
		pipeline.Spec.Input.Runtime.Namespaces = &v1beta1.NamespaceSelector{
			Exclude: md.ExcludeNamespaces,
		}
	}

	if pipeline.Spec.Input.Runtime != nil && pipeline.Spec.Input.Runtime.Resources == nil {
		pipeline.Spec.Input.Runtime.Resources = &v1beta1.MetricPipelineRuntimeInputResources{
			Pod: &v1beta1.MetricPipelineRuntimeInputResource{
				Enabled: md.RuntimeInputResources.Pod,
			},
			Container: &v1beta1.MetricPipelineRuntimeInputResource{
				Enabled: md.RuntimeInputResources.Container,
			},
			Node: &v1beta1.MetricPipelineRuntimeInputResource{
				Enabled: md.RuntimeInputResources.Node,
			},
			Volume: &v1beta1.MetricPipelineRuntimeInputResource{
				Enabled: md.RuntimeInputResources.Volume,
			},
			DaemonSet: &v1beta1.MetricPipelineRuntimeInputResource{
				Enabled: md.RuntimeInputResources.DaemonSet,
			},
			Deployment: &v1beta1.MetricPipelineRuntimeInputResource{
				Enabled: md.RuntimeInputResources.Deployment,
			},
			StatefulSet: &v1beta1.MetricPipelineRuntimeInputResource{
				Enabled: md.RuntimeInputResources.StatefulSet,
			},
			Job: &v1beta1.MetricPipelineRuntimeInputResource{
				Enabled: md.RuntimeInputResources.Job,
			},
		}
	}

	if pipeline.Spec.Input.Istio != nil && pipeline.Spec.Input.Istio.Namespaces == nil {
		pipeline.Spec.Input.Istio.Namespaces = &v1beta1.NamespaceSelector{
			Exclude: md.ExcludeNamespaces,
		}
	}

	if pipeline.Spec.Output.OTLP.Protocol == "" {
		pipeline.Spec.Output.OTLP.Protocol = md.DefaultOTLPOutputProtocol
	}
}
