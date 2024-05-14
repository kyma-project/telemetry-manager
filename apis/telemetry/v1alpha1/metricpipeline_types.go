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
//+kubebuilder:printcolumn:name="Configuration Generated",type=string,JSONPath=`.status.conditions[?(@.type=="ConfigurationGenerated")].status`
//+kubebuilder:printcolumn:name="Gateway Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="GatewayHealthy")].status`
//+kubebuilder:printcolumn:name="Agent Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="AgentHealthy")].status`
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
	//+optional
	Prometheus *MetricPipelinePrometheusInput `json:"prometheus,omitempty"`
	// Configures runtime scraping.
	//+optional
	Runtime *MetricPipelineRuntimeInput `json:"runtime,omitempty"`
	// Configures istio-proxy metrics scraping.
	//+optional
	Istio *MetricPipelineIstioInput `json:"istio,omitempty"`
	// Configures the collection of push-based metrics that use the OpenTelemetry protocol.
	//+optional
	Otlp *MetricPipelineOtlpInput `json:"otlp,omitempty"`
}

// MetricPipelinePrometheusInput defines the Prometheus scraping section.
type MetricPipelinePrometheusInput struct {
	// If enabled, Pods marked with `prometheus.io/scrape=true` annotation are scraped. The default is `false`.
	Enabled bool `json:"enabled,omitempty"`
	// Describes whether Prometheus metrics from specific Namespaces are selected. System Namespaces are disabled by default.
	//+optional
	//+kubebuilder:default={exclude: {kyma-system, kube-system, istio-system, compass-system}}
	Namespaces *MetricPipelineInputNamespaceSelector `json:"namespaces,omitempty"`
	// Configures diagnostic metrics scraping
	//+optional
	DiagnosticMetrics *DiagnosticMetrics `json:"diagnosticMetrics,omitempty"`
}

// MetricPipelineRuntimeInput defines the runtime scraping section.
type MetricPipelineRuntimeInput struct {
	// If enabled, workload-related Kubernetes metrics are scraped. The default is `false`.
	Enabled bool `json:"enabled,omitempty"`
	// Describes whether workload-related Kubernetes metrics from specific Namespaces are selected. System Namespaces are disabled by default.
	//+optional
	//+kubebuilder:default={exclude: {kyma-system, kube-system, istio-system, compass-system}}
	Namespaces *MetricPipelineInputNamespaceSelector `json:"namespaces,omitempty"`
}

// MetricPipelineIstioInput defines the Istio scraping section.
type MetricPipelineIstioInput struct {
	// If enabled, metrics for istio-proxy containers are scraped from Pods that have had the istio-proxy sidecar injected. The default is `false`.
	Enabled bool `json:"enabled,omitempty"`
	// Describes whether istio-proxy metrics from specific Namespaces are selected. System Namespaces are enabled by default.
	//+optional
	Namespaces *MetricPipelineInputNamespaceSelector `json:"namespaces,omitempty"`
	// Configures diagnostic metrics scraping
	//+optional
	DiagnosticMetrics *DiagnosticMetrics `json:"diagnosticMetrics,omitempty"`
}

// MetricPipelineOtlpInput defines the collection of push-based metrics that use the OpenTelemetry protocol.
type MetricPipelineOtlpInput struct {
	// If disabled, push-based OTLP metrics are not collected. The default is `false`.
	Disabled bool `json:"disabled,omitempty"`
	// Describes whether push-based OTLP metrics from specific Namespaces are selected. System Namespaces are enabled by default.
	//+optional
	Namespaces *MetricPipelineInputNamespaceSelector `json:"namespaces,omitempty"`
}

// MetricPipelineInputNamespaceSelector describes whether metrics from specific Namespaces are selected.
// +kubebuilder:validation:XValidation:rule="!((has(self.include) && size(self.include) != 0) && (has(self.exclude) && size(self.exclude) != 0))", message="Can only define one namespace selector - either 'include' or 'exclude'"
type MetricPipelineInputNamespaceSelector struct {
	// Include metrics from the specified Namespace names only.
	Include []string `json:"include,omitempty"`
	// Exclude metrics from the specified Namespace names only.
	Exclude []string `json:"exclude,omitempty"`
}

// MetricPipelineOutput defines the output configuration section.
type MetricPipelineOutput struct {
	// Defines an output using the OpenTelemetry protocol.
	Otlp *OtlpOutput `json:"otlp"`
}

// DiagnosticMetrics defines the diagnostic metrics configuration section
type DiagnosticMetrics struct {
	// If enabled, diagnostic metrics are scraped. The default is `false`.
	Enabled bool `json:"enabled,omitempty"`
}

// MetricPipelineStatus defines the observed state of MetricPipeline.
type MetricPipelineStatus struct {
	// An array of conditions describing the status of the pipeline.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
