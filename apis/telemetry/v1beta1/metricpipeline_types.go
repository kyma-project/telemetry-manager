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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories={kyma-telemetry,kyma-telemetry-pipelines}
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Configuration Generated",type=string,JSONPath=`.status.conditions[?(@.type=="ConfigurationGenerated")].status`
// +kubebuilder:printcolumn:name="Gateway Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="GatewayHealthy")].status`
// +kubebuilder:printcolumn:name="Agent Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="AgentHealthy")].status`
// +kubebuilder:printcolumn:name="Flow Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="TelemetryFlowHealthy")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:storageversion

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
	// +optional
	Prometheus *MetricPipelinePrometheusInput `json:"prometheus,omitempty"`
	// Configures runtime scraping.
	// +optional
	Runtime *MetricPipelineRuntimeInput `json:"runtime,omitempty"`
	// Configures istio-proxy metrics scraping.
	// +optional
	Istio *MetricPipelineIstioInput `json:"istio,omitempty"`
	// Configures the collection of push-based metrics that use the OpenTelemetry protocol.
	// +optional
	OTLP *MetricPipelineOTLPInput `json:"otlp,omitempty"`
}

// MetricPipelinePrometheusInput defines the Prometheus scraping section.
type MetricPipelinePrometheusInput struct {
	// If enabled, Pods marked with `prometheus.io/scrape=true` annotation are scraped. The default is `false`.
	Enabled bool `json:"enabled,omitempty"`
	// Describes whether Prometheus metrics from specific namespaces are selected. System namespaces are disabled by default.
	// +optional
	// +kubebuilder:default={exclude: {kyma-system, kube-system, istio-system, compass-system}}
	Namespaces *MetricPipelineInputNamespaceSelector `json:"namespaces,omitempty"`
	// Configures diagnostic metrics scraping
	// +optional
	DiagnosticMetrics *DiagnosticMetrics `json:"diagnosticMetrics,omitempty"`
}

// MetricPipelineRuntimeInput defines the runtime scraping section.
type MetricPipelineRuntimeInput struct {
	// If enabled, runtime metrics are scraped. The default is `false`.
	Enabled bool `json:"enabled,omitempty"`
	// Describes whether runtime metrics from specific namespaces are selected. System namespaces are disabled by default.
	// +optional
	// +kubebuilder:default={exclude: {kyma-system, kube-system, istio-system, compass-system}}
	Namespaces *MetricPipelineInputNamespaceSelector `json:"namespaces,omitempty"`
	// Describes the Kubernetes resources for which runtime metrics are scraped.
	// +optional
	// +kubebuilder:default={pod: {enabled: true}, container: {enabled: true}, node: {enabled: false}, volume: {enabled: false}}
	Resources *MetricPipelineRuntimeInputResources `json:"resources,omitempty"`
}

// MetricPipelineRuntimeInputResources describes the Kubernetes resources for which runtime metrics are scraped.
type MetricPipelineRuntimeInputResources struct {
	// Configures Pod runtime metrics scraping.
	// +optional
	// +kubebuilder:default={enabled: true}
	Pod *MetricPipelineRuntimeInputResourceEnabledByDefault `json:"pod,omitempty"`
	// Configures container runtime metrics scraping.
	// +optional
	// +kubebuilder:default={enabled: true}
	Container *MetricPipelineRuntimeInputResourceEnabledByDefault `json:"container,omitempty"`
	// Configures Node runtime metrics scraping.
	// +optional
	// +kubebuilder:default={enabled: false}
	Node *MetricPipelineRuntimeInputResourceDisabledByDefault `json:"node,omitempty"`
	// Configures Volume runtime metrics scraping.
	// +optional
	// +kubebuilder:default={enabled: false}
	Volume *MetricPipelineRuntimeInputResourceDisabledByDefault `json:"volume,omitempty"`
}

// MetricPipelineRuntimeInputResourceEnabledByDefault defines if the scraping of runtime metrics is enabled for a specific resource. The scraping is enabled by default.
type MetricPipelineRuntimeInputResourceEnabledByDefault struct {
	// If enabled, the runtime metrics for the resource are scraped. The default is `true`.
	// +optional
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`
}

// MetricPipelineRuntimeInputResourceDisabledByDefault defines if the scraping of runtime metrics is enabled for a specific resource. The scraping is disabled by default.
type MetricPipelineRuntimeInputResourceDisabledByDefault struct {
	// If enabled, the runtime metrics for the resource are scraped. The default is `false`.
	// +optional
	// +kubebuilder:default=false
	Enabled *bool `json:"enabled,omitempty"`
}

// MetricPipelineIstioInput defines the Istio scraping section.
type MetricPipelineIstioInput struct {
	// If enabled, metrics for istio-proxy containers are scraped from Pods that have had the istio-proxy sidecar injected. The default is `false`.
	Enabled bool `json:"enabled,omitempty"`
	// Describes whether istio-proxy metrics from specific namespaces are selected. System namespaces are enabled by default.
	// +optional
	Namespaces *MetricPipelineInputNamespaceSelector `json:"namespaces,omitempty"`
	// Configures diagnostic metrics scraping
	// +optional
	DiagnosticMetrics *DiagnosticMetrics `json:"diagnosticMetrics,omitempty"`
}

// MetricPipelineOTLPInput defines the collection of push-based metrics that use the OpenTelemetry protocol.
type MetricPipelineOTLPInput struct {
	// If disabled, push-based OTLP metrics are not collected. The default is `false`.
	Disabled bool `json:"disabled,omitempty"`
	// Describes whether push-based OTLP metrics from specific namespaces are selected. System namespaces are enabled by default.
	// +optional
	Namespaces *MetricPipelineInputNamespaceSelector `json:"namespaces,omitempty"`
}

// MetricPipelineInputNamespaceSelector describes whether metrics from specific namespaces are selected.
// +kubebuilder:validation:XValidation:rule="!((has(self.include) && size(self.include) != 0) && (has(self.exclude) && size(self.exclude) != 0))", message="Can only define one namespace selector - either 'include' or 'exclude'"
type MetricPipelineInputNamespaceSelector struct {
	// Include metrics from the specified Namespace names only.
	Include []string `json:"include,omitempty"`
	// Exclude metrics from the specified Namespace names only.
	Exclude []string `json:"exclude,omitempty"`
}

// DiagnosticMetrics defines the diagnostic metrics configuration section
type DiagnosticMetrics struct {
	// If enabled, diagnostic metrics are scraped. The default is `false`.
	Enabled bool `json:"enabled,omitempty"`
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

// +kubebuilder:object:root=true

// MetricPipelineList contains a list of MetricPipeline.
type MetricPipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MetricPipeline `json:"items"`
}

//nolint:gochecknoinits // SchemeBuilder's registration is required.
func init() {
	SchemeBuilder.Register(&MetricPipeline{}, &MetricPipelineList{})
}
