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

type State string

// Valid Module CR States.
const (
	// StateReady signifies Module CR is Ready and has been installed successfully.
	StateReady State = "Ready"

	// StateDeleting signifies Module CR is being deleted. This is the state that is used
	// when a deletionTimestamp was detected and Finalizers are picked up.
	StateDeleting State = "Deleting"

	// StateWarning signifies specified resource has been deployed, but cannot be used due to misconfiguration,
	// usually it means that user interaction is required.
	StateWarning State = "Warning"
)

// TelemetrySpec defines the desired state of Telemetry
type TelemetrySpec struct {
	// +optional
	Trace *TraceSpec `json:"trace,omitempty"`

	// +optional
	Metric *MetricSpec `json:"metric,omitempty"`
}

// MetricSpec defines the behavior of the metric gateway
type MetricSpec struct {
	Gateway MetricGatewaySpec `json:"gateway,omitempty"`
}

type MetricGatewaySpec struct {
	Scaling Scaling `json:"scaling,omitempty"`
}

// TraceSpec defines the behavior of the trace gateway
type TraceSpec struct {
	Gateway TraceGatewaySpec `json:"gateway,omitempty"`
}

type TraceGatewaySpec struct {
	Scaling Scaling `json:"scaling,omitempty"`
}

// Scaling defines which strategy is used for scaling the gateway, with detailed configuration options for each strategy type.
type Scaling struct {
	// Type of scaling strategy. Default is none, using a fixed amount of replicas.
	// +optional
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=Static
	Type ScalingStrategyType `json:"type,omitempty"`

	// Static is a scaling strategy allowing you to define a custom amount of replicas to be used for the gateway. Present only if Type =
	// StaticScalingStrategyType.
	// +optional
	Static *StaticScaling `json:"static,omitempty"`
}

// +enum
type ScalingStrategyType string

const (
	StaticScalingStrategyType ScalingStrategyType = "Static"
)

type StaticScaling struct {
	// Replicas defines a static number of pods to run the gateway. Minimum is 1.
	// +kubebuilder:validation:Minimum=1
	Replicas int32 `json:"replicas,omitempty"`
}

// TelemetryStatus defines the observed state of Telemetry
type TelemetryStatus struct {
	Status `json:",inline"`

	// Conditions contain a set of conditionals to determine the State of Status.
	// If all Conditions are met, State is expected to be in StateReady.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// endpoints for trace and metric gateway.
	// +nullable
	GatewayEndpoints GatewayEndpoints `json:"endpoints,omitempty"`
	// add other fields to status subresource here
}

type GatewayEndpoints struct {
	//traces contains the endpoints for trace gateway supporting OTLP.
	Traces *OTLPEndpoints `json:"traces,omitempty"`

	//metrics contains the endpoints for metric gateway supporting OTLP.
	Metrics *OTLPEndpoints `json:"metrics,omitempty"`
}

type OTLPEndpoints struct {
	//GRPC endpoint for OTLP.
	GRPC string `json:"grpc,omitempty"`
	//HTTP endpoint for OTLP.
	HTTP string `json:"http,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="generation",type="integer",JSONPath=".metadata.generation"
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="state",type="string",JSONPath=".status.state"
// Telemetry is the Schema for the telemetries API
type Telemetry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TelemetrySpec   `json:"spec,omitempty"`
	Status TelemetryStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TelemetryList contains a list of Telemetry
type TelemetryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Telemetry `json:"items"`
}

//nolint:gochecknoinits // SchemeBuilder's registration is required.
func init() {
	SchemeBuilder.Register(&Telemetry{}, &TelemetryList{})
}

// +k8s:deepcopy-gen=true

// Status defines the observed state of Module CR.
type Status struct {
	// State signifies current state of Module CR.
	// Value can be one of these three: "Ready", "Deleting", or "Warning".
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Deleting;Ready;Warning
	State State `json:"state"`
}
