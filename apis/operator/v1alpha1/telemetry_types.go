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
	// Trace configures module settings specific to the trace features. This field is optional.
	// +optional
	Trace *TraceSpec `json:"trace,omitempty"`

	// Metric configures module settings specific to the metric features. This field is optional.
	// +optional
	Metric *MetricSpec `json:"metric,omitempty"`

	// Log configures module settings specific to the log features. This field is optional.
	// +optional
	Log *LogSpec `json:"log,omitempty"`

	// Enrichments configures optional enrichments of all telemetry data collected by pipelines. This field is optional.
	// +optional
	Enrichments *EnrichmentSpec `json:"enrichments,omitempty"`
}

// MetricSpec configures module settings specific to the metric features.
type MetricSpec struct {
	// Gateway configures the metric gateway.
	Gateway GatewaySpec `json:"gateway,omitempty"`
}

// TraceSpec configures module settings specific to the trace features.
type TraceSpec struct {
	// Gateway configures the trace gateway.
	Gateway GatewaySpec `json:"gateway,omitempty"`
}

// LogSpec configures module settings specific to the log features.
type LogSpec struct {
	// Gateway configures the log gateway.
	Gateway GatewaySpec `json:"gateway,omitempty"`
}

// GatewaySpec defines settings of a gateway.
type GatewaySpec struct {
	// Scaling defines which strategy is used for scaling the gateway, with detailed configuration options for each strategy type.
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
	// +optional
	Static *StaticScaling `json:"static,omitempty"`
}

// +enum
type ScalingStrategyType string

const (
	StaticScalingStrategyType ScalingStrategyType = "Static"
)

type StaticScaling struct {
	// Replicas defines a static number of Pods to run the gateway. Minimum is 1.
	// +kubebuilder:validation:Minimum=1
	Replicas int32 `json:"replicas,omitempty"`
}

// TelemetryStatus defines the observed state of Telemetry
type TelemetryStatus struct {
	Status `json:",inline"`

	// Conditions contain a set of conditionals to determine the State of Status.
	// If all Conditions are met, State is expected to be in StateReady.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Endpoints for log, trace, and metric gateway.
	// +nullable
	Endpoints GatewayEndpoints `json:"endpoints,omitempty"`
	// add other fields to status subresource here
}

type GatewayEndpoints struct {
	// Logs contains the endpoints for log gateway supporting OTLP.
	Logs *OTLPEndpoints `json:"logs,omitempty"`

	// Traces contains the endpoints for trace gateway supporting OTLP.
	Traces *OTLPEndpoints `json:"traces,omitempty"`

	// Metrics contains the endpoints for metric gateway supporting OTLP.
	Metrics *OTLPEndpoints `json:"metrics,omitempty"`
}

type OTLPEndpoints struct {
	// gRPC endpoint for OTLP.
	GRPC string `json:"grpc,omitempty"`
	// HTTP endpoint for OTLP.
	HTTP string `json:"http,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,categories={kyma-modules,kyma-telemetry}
// +kubebuilder:metadata:labels={app.kubernetes.io/component=controller,app.kubernetes.io/managed-by=kyma,app.kubernetes.io/name=telemetry-manager,app.kubernetes.io/part-of=telemetry,kyma-project.io/module=telemetry}
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="state",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"

// Telemetry is the Schema for the telemetries API
type Telemetry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TelemetrySpec   `json:"spec,omitempty"`
	Status TelemetryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TelemetryList contains a list of Telemetry
type TelemetryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Telemetry `json:"items"`
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

// EnrichmentSpec defines the configuration for telemetry data enrichment.
// EnrichmentSpec contains settings to enable enrichment and specify pod labels for enrichment.
type EnrichmentSpec struct {
	// ExtractPodLabels specifies the list of Pod labels to be used for enrichment.
	// This field is optional.
	ExtractPodLabels []PodLabel `json:"extractPodLabels,omitempty"`

	// Cluster provides user-defined cluster definitions to enrich resource attributes.
	// +optional
	Cluster *Cluster `json:"cluster,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="(has(self.key) || has(self.keyPrefix))", message="Either 'key' or 'keyPrefix' must be specified"
// +kubebuilder:validation:XValidation:rule="!(has(self.key) && has(self.keyPrefix))", message="Either 'key' or 'keyPrefix' must be specified"
// PodLabel defines labels from a Pod used for telemetry data enrichments, which can be specified either by a key or a key prefix.
// Either 'key' or 'keyPrefix' must be specified, but not both.
// The enriched telemetry data contains resource attributes with key k8s.pod.label.<label_key>.
type PodLabel struct {
	// Key specifies the exact label key to be used.
	// This field is optional.
	Key string `json:"key,omitempty"`

	// KeyPrefix specifies a prefix for label keys to be used.
	// This field is optional.
	KeyPrefix string `json:"keyPrefix,omitempty"`
}

// Use Cluster to define custom cluster details to enrich your telemetry resource attributes.
type Cluster struct {
	// Name specifies a custom cluster name for the resource attribute `k8s.cluster.name`.
	Name string `json:"name"`
}
