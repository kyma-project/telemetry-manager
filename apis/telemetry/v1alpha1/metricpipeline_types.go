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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//nolint:gochecknoinits // SchemeBuilder's registration is required.
func init() {
	SchemeBuilder.Register(&MetricPipeline{}, &MetricPipelineList{})
}

//+kubebuilder:object:root=true

// MetricPipelineList contains a list of MetricPipeline.
type MetricPipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MetricPipeline `json:"items"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.conditions[-1].type`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// MetricPipeline is the Schema for the metricpipelines API.
type MetricPipeline struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Defines the desired characteristics of MetricPipeline.
	Spec MetricPipelineSpec `json:"spec,omitempty"`

	// Represents the current information/status of MetricPipeline.
	Status MetricPipelineStatus `json:"status,omitempty"`
}

// MetricPipelineSpec defines the desired state of MetricPipeline.
type MetricPipelineSpec struct {
	// Configures different inputs to send additional metrics to the metric gateway.
	Input MetricPipelineInput `json:"input,omitempty"`

	// Configures the metric gateway.
	Output MetricPipelineOutput `json:"output,omitempty"`
}

// MetricPipelineInput defines the input configuration section.
type MetricPipelineInput struct {
	// Configures Prometheus scraping.
	Prometheus MetricPipelinePrometheusInput `json:"prometheus,omitempty"`
	// Configures runtime scraping.
	Runtime MetricPipelineRuntimeInput `json:"runtime,omitempty"`
	// Configures istio-proxy metrics scraping.
	Istio MetricPipelineIstioInput `json:"istio,omitempty"`
	// Configures the collection of push-based metrics that use the OpenTelemetry protocol.
	Otlp MetricPipelineOtlpInput `json:"otlp,omitempty"`
}

// MetricPipelinePrometheusInput defines the Prometheus scraping section.
type MetricPipelinePrometheusInput struct {
	// If enabled, Pods marked with `prometheus.io/scrape=true` annotation are scraped. The default is `false`.
	Enabled *bool `json:"enabled,omitempty"`
	// Describes whether Prometheus metrics from specific Namespaces are selected. System Namespaces are disabled by default.
	Namespaces MetricPipelineInputNamespaceSelector `json:"namespaces,omitempty"`
}

// MetricPipelineRuntimeInput defines the runtime scraping section.
type MetricPipelineRuntimeInput struct {
	// If enabled, workload-related Kubernetes metrics are scraped. The default is `false`.
	Enabled *bool `json:"enabled,omitempty"`
	// Describes whether workload-related Kubernetes metrics from specific Namespaces are selected. System Namespaces are disabled by default.
	Namespaces MetricPipelineInputNamespaceSelector `json:"namespaces,omitempty"`
}

// MetricPipelineIstioInput defines the Istio scraping section.
type MetricPipelineIstioInput struct {
	// If enabled, metrics for istio-proxy containers are scraped from Pods that have had the istio-proxy sidecar injected. The default is `false`.
	Enabled *bool `json:"enabled,omitempty"`
	// Describes whether istio-proxy metrics from specific Namespaces are selected. System Namespaces are enabled by default.
	Namespaces MetricPipelineInputNamespaceSelector `json:"namespaces,omitempty"`
}

// MetricPipelineOtlpInput defines the collection of push-based metrics that use the OpenTelemetry protocol.
type MetricPipelineOtlpInput struct {
	// If enabled, push-based OTLP metrics are collected. The default is `true`.
	Enabled *bool `json:"enabled,omitempty"`
	// Describes whether push-based OTLP metrics from specific Namespaces are selected. System namespaces are disabled by default.
	Namespaces MetricPipelineInputNamespaceSelector `json:"namespaces,omitempty"`
}

// MetricPipelineInputNamespaceSelector describes whether metrics from specific Namespaces are selected.
// +kubebuilder:validation:XValidation:rule="!((has(self.include) && size(self.include) != 0) && (has(self.exclude) && size(self.exclude) != 0))", message="Can only define one namespace selector - either 'include', 'exclude', or 'system'"
// +kubebuilder:validation:XValidation:rule="!((has(self.include) && size(self.include) != 0) && has(self.system))", message="Can only define one namespace selector - either 'include', 'exclude', or 'system'"
// +kubebuilder:validation:XValidation:rule="!((has(self.exclude) && size(self.exclude) != 0) && has(self.system))", message="Can only define one namespace selector - either 'include', 'exclude', or 'system'"
type MetricPipelineInputNamespaceSelector struct {
	// Include metrics from the specified Namespace names only.
	Include []string `json:"include,omitempty"`
	// Exclude metrics from the specified Namespace names only.
	Exclude []string `json:"exclude,omitempty"`
	// Set to `true` to include the metrics from system Namespaces like kube-system, istio-system, and kyma-system.
	System *bool `json:"system,omitempty"`
}

// MetricPipelineOutput defines the output configuration section.
type MetricPipelineOutput struct {
	// Defines an output using the OpenTelemetry protocol.
	Otlp *OtlpOutput `json:"otlp"`
}

// MetricPipelineStatus defines the observed state of MetricPipeline.
type MetricPipelineStatus struct {
	// An array of conditions describing the status of the pipeline.
	Conditions []MetricPipelineCondition `json:"conditions,omitempty"`
}

type MetricPipelineConditionType string

// These are the valid statuses of MetricPipeline.
const (
	MetricPipelinePending MetricPipelineConditionType = "Pending"
	MetricPipelineRunning MetricPipelineConditionType = "Running"
)

// MetricPipelineCondition contains details for the current condition of this LogPipeline.
type MetricPipelineCondition struct {
	// Point in time the condition transitioned into a different state.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Reason of last transition.
	Reason string `json:"reason,omitempty"`
	// The possible transition types are:<br>- `Running`: The instance is ready and usable.<br>- `Pending`: The pipeline is being activated.
	Type MetricPipelineConditionType `json:"type,omitempty"`
}

func NewMetricPipelineCondition(reason string, condType MetricPipelineConditionType) *MetricPipelineCondition {
	return &MetricPipelineCondition{
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Type:               condType,
	}
}

func (mps *MetricPipelineStatus) GetCondition(condType MetricPipelineConditionType) *MetricPipelineCondition {
	for cond := range mps.Conditions {
		if mps.Conditions[cond].Type == condType {
			return &mps.Conditions[cond]
		}
	}
	return nil
}

func (mps *MetricPipelineStatus) HasCondition(condition MetricPipelineConditionType) bool {
	return mps.GetCondition(condition) != nil
}

func (mps *MetricPipelineStatus) SetCondition(cond MetricPipelineCondition) {
	currentCond := mps.GetCondition(cond.Type)
	if currentCond != nil && currentCond.Reason == cond.Reason {
		return
	}
	if currentCond != nil {
		cond.LastTransitionTime = currentCond.LastTransitionTime
	}
	newConditions := filterMetricPipelineCondition(mps.Conditions, cond.Type)
	mps.Conditions = append(newConditions, cond)
}

func filterMetricPipelineCondition(conditions []MetricPipelineCondition, condType MetricPipelineConditionType) []MetricPipelineCondition {
	var newConditions []MetricPipelineCondition
	for _, cond := range conditions {
		if cond.Type == condType {
			continue
		}
		newConditions = append(newConditions, cond)
	}
	return newConditions
}
