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

// +kubebuilder:object:root=true
// MetricPipelineList contains a list of MetricPipeline.
type MetricPipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []MetricPipeline `json:"items"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories={kyma-telemetry,kyma-telemetry-pipelines}
// +kubebuilder:metadata:labels={app.kubernetes.io/component=controller,app.kubernetes.io/managed-by=kyma,app.kubernetes.io/name=telemetry-manager,app.kubernetes.io/part-of=telemetry,kyma-project.io/module=telemetry}
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Configuration Generated",type=string,JSONPath=`.status.conditions[?(@.type=="ConfigurationGenerated")].status`
// +kubebuilder:printcolumn:name="Gateway Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="GatewayHealthy")].status`
// +kubebuilder:printcolumn:name="Agent Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="AgentHealthy")].status`
// +kubebuilder:printcolumn:name="Flow Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="TelemetryFlowHealthy")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// MetricPipeline is the Schema for the metricpipelines API.
type MetricPipeline struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired characteristics of MetricPipeline.
	Spec MetricPipelineSpec `json:"spec,omitempty"`

	// Status represents the current information/status of MetricPipeline.
	Status MetricPipelineStatus `json:"status,omitempty"`
}

// MetricPipelineSpec defines the desired state of MetricPipeline.
type MetricPipelineSpec struct {
	// Input configures additional inputs for metric collection.
	Input MetricPipelineInput `json:"input,omitempty"`

	// Output configures the backend to which metrics are sent. You must specify exactly one output per pipeline.
	Output MetricPipelineOutput `json:"output,omitempty"`

	// Transforms specify a list of transformations to apply to telemetry data.
	// +optional
	Transforms []TransformSpec `json:"transform,omitempty"`
}

// MetricPipelineInput configures additional inputs for metric collection.
type MetricPipelineInput struct {
	// Prometheus input configures collection of application metrics in the pull-based Prometheus protocol using endpoint discovery based on annotations.
	// +optional
	Prometheus *MetricPipelinePrometheusInput `json:"prometheus,omitempty"`
	// Runtime input configures collection of Kubernetes runtime metrics.
	// +optional
	Runtime *MetricPipelineRuntimeInput `json:"runtime,omitempty"`
	// Istio input configures collection of Istio metrics from applications running in the Istio service mesh.
	// +optional
	Istio *MetricPipelineIstioInput `json:"istio,omitempty"`
	// OTLP input configures the push endpoint to receive metrics from an OTLP source.
	// +optional
	OTLP *OTLPInput `json:"otlp,omitempty"`
}

// MetricPipelinePrometheusInput collection of application metrics in the pull-based Prometheus protocol using endpoint discovery based on annotations.
type MetricPipelinePrometheusInput struct {
	// Enabled specifies whether Service endpoints and Pods marked with `prometheus.io/scrape=true` annotation are scraped. The default is `false`.
	Enabled *bool `json:"enabled,omitempty"`
	// Namespaces specifies from which namespaces metrics are collected. By default, all namespaces except the system namespaces are enabled. To enable all namespaces including system namespaces, use an empty struct notation.
	// +optional
	Namespaces *NamespaceSelector `json:"namespaces,omitempty"`
	// DiagnosticMetrics configures collection of additional diagnostic metrics. The default is `false`.
	DiagnosticMetrics *MetricPipelineIstioInputDiagnosticMetrics `json:"diagnosticMetrics,omitempty"`
}

// MetricPipelineRuntimeInput configures collection of Kubernetes runtime metrics.
type MetricPipelineRuntimeInput struct {
	// Enabled specifies whether runtime metrics are collected. The default is `false`.
	Enabled *bool `json:"enabled,omitempty"`
	// Namespaces specifies from which namespaces metrics are collected. By default, all namespaces except the system namespaces are enabled. To enable all namespaces including system namespaces, use an empty struct notation.
	// +optional
	Namespaces *NamespaceSelector `json:"namespaces,omitempty"`
	// Resources configures the Kubernetes resource types for which metrics are collected.
	// +optional
	Resources *MetricPipelineRuntimeInputResources `json:"resources,omitempty"`
}

// MetricPipelineRuntimeInputResources configures the Kubernetes resource types for which metrics are collected.
type MetricPipelineRuntimeInputResources struct {
	// Pod configures Pod runtime metrics collection.
	// +optional
	Pod *MetricPipelineRuntimeInputResource `json:"pod,omitempty"`
	// Container configures container runtime metrics collection.
	// +optional
	Container *MetricPipelineRuntimeInputResource `json:"container,omitempty"`
	// Node configures Node runtime metrics collection.
	// +optional
	Node *MetricPipelineRuntimeInputResource `json:"node,omitempty"`
	// Volume configures Volume runtime metrics collection.
	// +optional
	Volume *MetricPipelineRuntimeInputResource `json:"volume,omitempty"`
	// DaemonSet configures DaemonSet runtime metrics collection.
	// +optional
	DaemonSet *MetricPipelineRuntimeInputResource `json:"daemonset,omitempty"`
	// Deployment configures Deployment runtime metrics collection.
	// +optional
	Deployment *MetricPipelineRuntimeInputResource `json:"deployment,omitempty"`
	// StatefulSet configures StatefulSet runtime metrics collection.
	// +optional
	StatefulSet *MetricPipelineRuntimeInputResource `json:"statefulset,omitempty"`
	// Job configures Job runtime metrics collection.
	// +optional
	Job *MetricPipelineRuntimeInputResource `json:"job,omitempty"`
}

// MetricPipelineRuntimeInputResource configures if the collection of runtime metrics is enabled for a specific resource type. The collection is enabled by default.
type MetricPipelineRuntimeInputResource struct {
	// Enabled specifies that the runtime metrics for the resource type are collected. The default is `true`.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// MetricPipelineIstioInput defines the Istio scraping section.
type MetricPipelineIstioInput struct {
	// Enabled specifies that istio-proxy metrics are scraped from Pods that have the istio-proxy sidecar injected. The default is `false`.
	Enabled *bool `json:"enabled,omitempty"`
	// Namespaces configures the namespaces for which the collection should be activated. By default, all namespaces including system namespaces are enabled.
	Namespaces *NamespaceSelector `json:"namespaces,omitempty"`
	// DiagnosticMetrics configures collection of additional diagnostic metrics. The default is `false`.
	DiagnosticMetrics *MetricPipelineIstioInputDiagnosticMetrics `json:"diagnosticMetrics,omitempty"`
	// EnvoyMetrics enables the collection of additional Envoy metrics with prefix `envoy_`. The default is `false`.
	EnvoyMetrics *EnvoyMetrics `json:"envoyMetrics,omitempty"`
}

// MetricPipelineIstioInputDiagnosticMetrics defines the diagnostic metrics configuration section
type MetricPipelineIstioInputDiagnosticMetrics struct {
	// If enabled, diagnostic metrics are collected. The default is `false`.
	Enabled *bool `json:"enabled,omitempty"`
}

// MetricPipelineOutput defines the output configuration section.
type MetricPipelineOutput struct {
	// OTLP output defines an output using the OpenTelemetry protocol.
	OTLP *OTLPOutput `json:"otlp"`
}

// MetricPipelineStatus defines the observed state of MetricPipeline.
type MetricPipelineStatus struct {
	// An array of conditions describing the status of the pipeline.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// EnvoyMetrics defines the configuration for scraping Envoy metrics.
type EnvoyMetrics struct {
	// Enabled specifies that Envoy metrics with prefix `envoy_` are scraped additionally. The default is `false`.
	Enabled *bool `json:"enabled,omitempty"`
}
