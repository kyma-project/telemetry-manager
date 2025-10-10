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
	SchemeBuilder.Register(&Telemetry{}, &TelemetryList{})
}

// TelemetryList contains a list of Telemetry
// +kubebuilder:object:root=true
type TelemetryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Telemetry `json:"items"`
}

// Telemetry is the Schema for the telemetries API
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,categories={kyma-modules,kyma-telemetry}
// +kubebuilder:metadata:labels={app.kubernetes.io/component=controller,app.kubernetes.io/managed-by=kyma,app.kubernetes.io/name=telemetry-manager,app.kubernetes.io/part-of=telemetry,kyma-project.io/module=telemetry}
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="state",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"
type Telemetry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TelemetrySpec   `json:"spec,omitempty"`
	Status TelemetryStatus `json:"status,omitempty"`
}

// TelemetrySpec defines the desired state of Telemetry
type TelemetrySpec struct {
	// Trace configures module settings specific to the trace features. This field is optional.
	// +kubebuilder:validation:Optional
	Trace *TraceSpec `json:"trace,omitempty"`

	// Metric configures module settings specific to the metric features. This field is optional.
	// +kubebuilder:validation:Optional
	Metric *MetricSpec `json:"metric,omitempty"`

	// Log configures module settings specific to the log features. This field is optional.
	// +kubebuilder:validation:Optional
	Log *LogSpec `json:"log,omitempty"`

	// Enrichments configures optional enrichments of all telemetry data collected by pipelines. This field is optional.
	// +kubebuilder:validation:Optional
	Enrichments *EnrichmentSpec `json:"enrichments,omitempty"`
}

// MetricSpec configures module settings specific to the metric features.
type MetricSpec struct {
	// Gateway configures the metric gateway.
	// +kubebuilder:validation:Optional
	Gateway GatewaySpec `json:"gateway,omitempty"`
}

// TraceSpec configures module settings specific to the trace features.
type TraceSpec struct {
	// Gateway configures the trace gateway.
	// +kubebuilder:validation:Optional
	Gateway GatewaySpec `json:"gateway,omitempty"`
}

// LogSpec configures module settings specific to the log features.
type LogSpec struct {
	// Gateway configures the log gateway.
	// +kubebuilder:validation:Optional
	Gateway GatewaySpec `json:"gateway,omitempty"`
}

// GatewaySpec defines settings of a gateway.
type GatewaySpec struct {
	// Scaling defines which strategy is used for scaling the gateway, with detailed configuration options for each strategy type.
	// +kubebuilder:validation:Optional
	Scaling Scaling `json:"scaling,omitempty"`
}

// Scaling defines which strategy is used for scaling the gateway, with detailed configuration options for each strategy type.
type Scaling struct {
	// Type of scaling strategy. Default is none, using a fixed amount of replicas.
	// +optional
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=Static
	Type ScalingStrategyType `json:"type,omitempty"`

	// Static is a scaling strategy enabling you to define a custom amount of replicas to be used for the gateway. Present only if Type =
	// StaticScalingStrategyType.
	// +kubebuilder:validation:Optional
	Static *StaticScaling `json:"static,omitempty"`
}

// EnrichmentSpec defines settings to configure enrichment and specify pod labels for enrichment.
type EnrichmentSpec struct {
	// ExtractPodLabels specifies the list of Pod labels to be used for enrichment.
	// +kubebuilder:validation:Optional
	ExtractPodLabels []PodLabel `json:"extractPodLabels,omitempty"`

	// Cluster provides user-defined cluster definitions to enrich resource attributes.
	// +kubebuilder:validation:Optional
	Cluster *Cluster `json:"cluster,omitempty"`
}

// PodLabel defines labels from a Pod used for telemetry data enrichments, which can be specified either by a key or a key prefix.
// Either 'key' or 'keyPrefix' must be specified, but not both.
// The enriched telemetry data contains resource attributes with key k8s.pod.label.<label_key>.
// +kubebuilder:validation:XValidation:rule="(has(self.key) || has(self.keyPrefix))", message="Either 'key' or 'keyPrefix' must be specified"
// +kubebuilder:validation:XValidation:rule="!(has(self.key) && has(self.keyPrefix))", message="Either 'key' or 'keyPrefix' must be specified"
type PodLabel struct {
	// Key specifies the exact label key to be used.
	// +kubebuilder:validation:Optional
	Key string `json:"key,omitempty"`

	// KeyPrefix specifies a prefix for label keys to be used.
	// +kubebuilder:validation:Optional
	KeyPrefix string `json:"keyPrefix,omitempty"`
}

// Cluster defines custom cluster details to enrich your telemetry resource attributes.
type Cluster struct {
	// Name specifies a custom cluster name for the resource attribute `k8s.cluster.name`.
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`
}

// ScalingStrategyType defines the available scaling strategies available for a gateway.
// Currently the only supported strategy is Static, which allows defining a fixed number of replicas.
// +enum
type ScalingStrategyType string

const (
	StaticScalingStrategyType ScalingStrategyType = "Static"
)

type StaticScaling struct {
	// Replicas defines a static number of Pods to run the gateway. Minimum is 1.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Optional
	Replicas int32 `json:"replicas,omitempty"`
}

// TelemetryStatus defines the observed state of Telemetry
type TelemetryStatus struct {
	Status `json:",inline"`

	// Conditions contain a set of conditionals to determine the State of Status.
	// If all Conditions are met, State is expected to be in StateReady.
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Endpoints for log, trace, and metric gateway.
	// +nullable
	Endpoints GatewayEndpoints `json:"endpoints,omitempty"`
	// +kubebuilder:validation:Optional
	// add other fields to status subresource here
}

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

// Status defines the observed state of Module CR.
// +k8s:deepcopy-gen=true
type Status struct {
	// State signifies current state of Module CR.
	// Value can be one of these three: "Ready", "Deleting", or "Warning".
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Deleting;Ready;Warning
	State State `json:"state"`
}

type GatewayEndpoints struct {
	// Logs contains the endpoints for log gateway supporting OTLP.
	// +kubebuilder:validation:Optional
	Logs *OTLPEndpoints `json:"logs,omitempty"`

	// Traces contains the endpoints for trace gateway supporting OTLP.
	// +kubebuilder:validation:Optional
	Traces *OTLPEndpoints `json:"traces,omitempty"`

	// Metrics contains the endpoints for metric gateway supporting OTLP.
	// +kubebuilder:validation:Optional
	Metrics *OTLPEndpoints `json:"metrics,omitempty"`
}

type OTLPEndpoints struct {
	// gRPC endpoint for OTLP.
	// +kubebuilder:validation:Optional
	GRPC string `json:"grpc,omitempty"`
	// HTTP endpoint for OTLP.
	// +kubebuilder:validation:Optional
	HTTP string `json:"http,omitempty"`
}
