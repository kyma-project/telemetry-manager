package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//nolint:gochecknoinits // SchemeBuilder's registration is required.
func init() {
	SchemeBuilder.Register(&TracePipeline{}, &TracePipelineList{})
}

// TracePipelineList contains a list of TracePipeline
// +kubebuilder:object:root=true
type TracePipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []TracePipeline `json:"items"`
}

// TracePipeline is the Schema for the tracepipelines API
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories={kyma-telemetry,kyma-telemetry-pipelines}
// +kubebuilder:metadata:labels={app.kubernetes.io/component=controller,app.kubernetes.io/managed-by=kyma,app.kubernetes.io/name=telemetry-manager,app.kubernetes.io/part-of=telemetry,kyma-project.io/module=telemetry}
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Configuration Generated",type=string,JSONPath=`.status.conditions[?(@.type=="ConfigurationGenerated")].status`
// +kubebuilder:printcolumn:name="Gateway Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="GatewayHealthy")].status`
// +kubebuilder:printcolumn:name="Flow Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="TelemetryFlowHealthy")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:storageversion
type TracePipeline struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of TracePipeline
	// +kubebuilder:validation:Optional
	Spec TracePipelineSpec `json:"spec,omitempty"`
	// Status shows the observed state of the TracePipeline
	// +kubebuilder:validation:Optional
	Status TracePipelineStatus `json:"status,omitempty"`
}

// TracePipelineSpec defines the desired state of TracePipeline
type TracePipelineSpec struct {
	// Output configures the backend to which traces are sent. You must specify exactly one output per pipeline.
	// +kubebuilder:validation:Required
	Output TracePipelineOutput `json:"output"`

	// Transforms specify a list of transformations to apply to telemetry data.
	// +kubebuilder:validation:Optional
	Transforms []TransformSpec `json:"transform,omitempty"`

	// Filter specifies a list of filters to apply to telemetry data.
	// +kubebuilder:validation:Optional
	Filters []FilterSpec `json:"filter,omitempty"`
}

// TracePipelineOutput defines the output configuration section.
type TracePipelineOutput struct {
	// OTLP output defines an output using the OpenTelemetry protocol.
	// +kubebuilder:validation:Required
	OTLP *OTLPOutput `json:"otlp"`
}

// TracePipelineStatus defines the observed state of TracePipeline.
type TracePipelineStatus struct {
	// An array of conditions describing the status of the pipeline.
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
