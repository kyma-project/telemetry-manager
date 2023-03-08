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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MetricPipelineOutput defines the output configuration section.
type MetricPipelineOutput struct {
	// Defines an output using the OpenTelmetry protocol.
	Otlp *OtlpOutput `json:"otlp"`
}

// MetricPipelineSpec defines the desired state of MetricPipeline
type MetricPipelineSpec struct {
	// Configures the trace receiver of a MetricPipeline.
	Output MetricPipelineOutput `json:"output"`
}

type MetricPipelineConditionType string

// These are the valid statuses of MetricPipeline.
const (
	MetricPipelinePending MetricPipelineConditionType = "Pending"
	MetricPipelineRunning MetricPipelineConditionType = "Running"
)

// Contains details for the current condition of this MetricPipeline
type MetricPipelineCondition struct {
	LastTransitionTime metav1.Time                 `json:"lastTransitionTime,omitempty"`
	Reason             string                      `json:"reason,omitempty"`
	Type               MetricPipelineConditionType `json:"type,omitempty"`
}

// Defines the observed state of MetricPipeline
type MetricPipelineStatus struct {
	Conditions []MetricPipelineCondition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:subresource:status

// MetricPipeline is the Schema for the metricpipelines API
type MetricPipeline struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MetricPipelineSpec   `json:"spec,omitempty"`
	Status MetricPipelineStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MetricPipelineList contains a list of MetricPipeline
type MetricPipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MetricPipeline `json:"items"`
}

//nolint:gochecknoinits
func init() {
	SchemeBuilder.Register(&MetricPipeline{}, &MetricPipelineList{})
}
