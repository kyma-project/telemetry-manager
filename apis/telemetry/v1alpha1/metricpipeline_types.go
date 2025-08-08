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

	// Defines the desired characteristics of MetricPipeline.
	Spec MetricPipelineSpec `json:"spec,omitempty"`

	// Represents the current information/status of MetricPipeline.
	Status MetricPipelineStatus `json:"status,omitempty"`
}

// MetricPipelineSpec defines the desired state of MetricPipeline.
type MetricPipelineSpec struct {
	// Configures additional inputs for metric collection.
	Input MetricPipelineInput `json:"input,omitempty"`

	// Configures the output where the metrics will be send to. Exactly one output must be specified.
	Output MetricPipelineOutput `json:"output,omitempty"`
}

// MetricPipelineInput configures additional inputs for metric collection.
type MetricPipelineInput struct {
	// Configures collection of application metrics in the pull-based Prometheus protocol using endpoint discovery based on annotations.
	// +optional
	Prometheus *MetricPipelinePrometheusInput `json:"prometheus,omitempty"`
	// Configures collection of Kubernetes runtime metrics.
	// +optional
	Runtime *MetricPipelineRuntimeInput `json:"runtime,omitempty"`
	// Configures collection of Istio metrics from applications running in the Istio service mesh.
	// +optional
	Istio *MetricPipelineIstioInput `json:"istio,omitempty"`
	// Configures the push endpoint to receive metrics from a OTLP source.
	// +optional
	OTLP *OTLPInput `json:"otlp,omitempty"`
}

// MetricPipelinePrometheusInput collection of application metrics in the pull-based Prometheus protocol using endpoint discovery based on annotations.
type MetricPipelinePrometheusInput struct {
	// If enabled, Services endpoints and Pods marked with `prometheus.io/scrape=true` annotation are scraped. The default is `false`.
	Enabled *bool `json:"enabled,omitempty"`
	// Configures the namespaces for which the collection should be activated. All namespaces except the system namespaces are enabled by default, use an empty struct notation to enable all namespaces.
	// +optional
	Namespaces *NamespaceSelector `json:"namespaces,omitempty"`
	// Configures collection of additional diagnostic metrics. The default is `false`.
	DiagnosticMetrics *MetricPipelineIstioInputDiagnosticMetrics `json:"diagnosticMetrics,omitempty"`
}

// MetricPipelineRuntimeInput configures collection of Kubernetes runtime metrics.
type MetricPipelineRuntimeInput struct {
	// If enabled, runtime metrics are collected. The default is `false`.
	Enabled *bool `json:"enabled,omitempty"`
	// Configures the namespaces for which the collection should be activated. All namespaces except the system namespaces are enabled by default, use an empty struct notation to enable all namespaces.
	// +optional
	Namespaces *NamespaceSelector `json:"namespaces,omitempty"`
	// Configures the Kubernetes resource types for which metrics are collected.
	// +optional
	Resources *MetricPipelineRuntimeInputResources `json:"resources,omitempty"`
}

// MetricPipelineRuntimeInputResources configures the Kubernetes resource types for which metrics are collected.
type MetricPipelineRuntimeInputResources struct {
	// Configures Pod runtime metrics collection.
	// +optional
	Pod *MetricPipelineRuntimeInputResource `json:"pod,omitempty"`
	// Configures container runtime metrics collection.
	// +optional
	Container *MetricPipelineRuntimeInputResource `json:"container,omitempty"`
	// Configures Node runtime metrics collection.
	// +optional
	Node *MetricPipelineRuntimeInputResource `json:"node,omitempty"`
	// Configures Volume runtime metrics collection.
	// +optional
	Volume *MetricPipelineRuntimeInputResource `json:"volume,omitempty"`
	// Configures DaemonSet runtime metrics collection.
	// +optional
	DaemonSet *MetricPipelineRuntimeInputResource `json:"daemonset,omitempty"`
	// Configures Deployment runtime metrics collection.
	// +optional
	Deployment *MetricPipelineRuntimeInputResource `json:"deployment,omitempty"`
	// Configures StatefulSet runtime metrics collection.
	// +optional
	StatefulSet *MetricPipelineRuntimeInputResource `json:"statefulset,omitempty"`
	// Configures Job runtime metrics collection.
	// +optional
	Job *MetricPipelineRuntimeInputResource `json:"job,omitempty"`
}

// MetricPipelineRuntimeInputResource configures if the collection of runtime metrics is enabled for a specific resource type. The collection is enabled by default.
type MetricPipelineRuntimeInputResource struct {
	// If enabled, the runtime metrics for the resource type are collected. The default is `true`.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// MetricPipelineIstioInput defines the Istio scraping section.
type MetricPipelineIstioInput struct {
	// If enabled, istio-proxy metrics are scraped from Pods that have the istio-proxy sidecar injected. The default is `false`.
	Enabled *bool `json:"enabled,omitempty"`
	// Configures the namespaces for which the collection should be activated. All namespaces including system namespaces are enabled by default.
	// +optional
	Namespaces *NamespaceSelector `json:"namespaces,omitempty"`
	// Configures collection of additional diagnostic metrics. The default is `false`.
	DiagnosticMetrics *MetricPipelineIstioInputDiagnosticMetrics `json:"diagnosticMetrics,omitempty"`
	// If enabled, Envoy metrics with prefix `envoy_` are scraped additional. The default is `false`.
	EnvoyMetrics *EnvoyMetrics `json:"envoyMetrics,omitempty"`
}

// MetricPipelineIstioInputDiagnosticMetrics defines the diagnostic metrics configuration section
type MetricPipelineIstioInputDiagnosticMetrics struct {
	// If enabled, diagnostic metrics are collected. The default is `false`.
	Enabled *bool `json:"enabled,omitempty"`
}

// MetricPipelineOutput defines the output configuration section.
type MetricPipelineOutput struct {
	// Defines an output using the OpenTelemetry protocol.
	OTLP *OTLPOutput `json:"otlp"`
}

// MetricPipelineStatus defines the observed state of MetricPipeline.
type MetricPipelineStatus struct {
	// An array of conditions describing the status of the pipeline.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// EnvoyMetrics defines the configuration for scraping Envoy metrics.
type EnvoyMetrics struct {
	// If enabled, Envoy metrics with prefix `envoy_` are scraped additional. The default is `false`.
	Enabled *bool `json:"enabled,omitempty"`
}
