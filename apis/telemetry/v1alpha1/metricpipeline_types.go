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
	// Configures application related scraping.
	Application MetricPipelineApplicationInput `json:"application,omitempty"`
}

// MetricPipelineApplicationInput defines the application input configuration section.
type MetricPipelineApplicationInput struct {
	// Configures workload scraping.
	Workloads MetricPipelineWorkloadsInput `json:"workloads,omitempty"`
	// Configures runtime scraping (workload-related k8s ).
	Runtime MetricPipelineContainerRuntimeInput `json:"runtime,omitempty"`
}

// MetricPipelineWorkloadsInput defines the workload scraping section.
type MetricPipelineWorkloadsInput struct {
	// Indicates if workload scraping is enabled. Services and pods marked with prometheus.io/scrape=true annotation will be scraped.
	Enabled bool `json:"enabled,omitempty"`
}

// MetricPipelineContainerRuntimeInput defines the runtime scraping (kubelet, node metrics) section.
type MetricPipelineContainerRuntimeInput struct {
	// Indicates if runtime scraping is enabled.
	Enabled bool `json:"enabled,omitempty"`
}

// MetricPipelineIstioInput defines the Istio scraping section.
type MetricPipelineIstioInput struct {
	// Indicates if Istio scraping is enabled.
	Enabled bool `json:"enabled,omitempty"`
}

// MetricPipelineOutput defines the output configuration section.
type MetricPipelineOutput struct {
	// Defines an output using the OpenTelemetry protocol.
	Otlp *OtlpOutput `json:"otlp"`
}

// MetricPipelineStatus defines the observed state of MetricPipeline.
type MetricPipelineStatus struct {
	// Defines the trail of MetricPipeline conditions.
	Conditions []MetricPipelineCondition `json:"conditions,omitempty"`
}

type MetricPipelineConditionType string

// These are the valid statuses of MetricPipeline.
const (
	MetricPipelinePending MetricPipelineConditionType = "Pending"
	MetricPipelineRunning MetricPipelineConditionType = "Running"
)

// Contains details for the current condition of this MetricPipeline.
type MetricPipelineCondition struct {
	LastTransitionTime metav1.Time                 `json:"lastTransitionTime,omitempty"`
	Reason             string                      `json:"reason,omitempty"`
	Type               MetricPipelineConditionType `json:"type,omitempty"`
}

func NewMetricPipelineCondition(reason string, condType MetricPipelineConditionType) *MetricPipelineCondition {
	return &MetricPipelineCondition{
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Type:               condType,
	}
}

func (tps *MetricPipelineStatus) GetCondition(condType MetricPipelineConditionType) *MetricPipelineCondition {
	for cond := range tps.Conditions {
		if tps.Conditions[cond].Type == condType {
			return &tps.Conditions[cond]
		}
	}
	return nil
}

func (tps *MetricPipelineStatus) HasCondition(condition MetricPipelineConditionType) bool {
	return tps.GetCondition(condition) != nil
}

func (tps *MetricPipelineStatus) SetCondition(cond MetricPipelineCondition) {
	currentCond := tps.GetCondition(cond.Type)
	if currentCond != nil && currentCond.Reason == cond.Reason {
		return
	}
	if currentCond != nil {
		cond.LastTransitionTime = currentCond.LastTransitionTime
	}
	newConditions := filterMetricPipelineCondition(tps.Conditions, cond.Type)
	tps.Conditions = append(newConditions, cond)
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
