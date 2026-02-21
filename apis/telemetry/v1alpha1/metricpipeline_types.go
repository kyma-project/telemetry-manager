package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//nolint:gochecknoinits // SchemeBuilder's registration is required.
func init() {
	SchemeBuilder.Register(&MetricPipeline{}, &MetricPipelineList{})
}

// MetricPipelineList contains a list of MetricPipeline.
// +kubebuilder:object:root=true
type MetricPipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []MetricPipeline `json:"items"`
}

// MetricPipeline is the Schema for the metricpipelines API.
// +kubebuilder:object:root=true
// +kubebuilder:deprecatedversion:warning="telemetry.kyma-project.io/v1alpha1 MetricPipeline is deprecated. Use telemetry.kyma-project.io/v1beta1 MetricPipeline instead."
// +kubebuilder:resource:scope=Cluster,categories={kyma-telemetry,kyma-telemetry-pipelines}
// +kubebuilder:metadata:labels={app.kubernetes.io/component=controller,app.kubernetes.io/managed-by=kyma,app.kubernetes.io/name=telemetry-manager,app.kubernetes.io/part-of=telemetry,kyma-project.io/module=telemetry}
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Configuration Generated",type=string,JSONPath=`.status.conditions[?(@.type=="ConfigurationGenerated")].status`
// +kubebuilder:printcolumn:name="Gateway Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="GatewayHealthy")].status`
// +kubebuilder:printcolumn:name="Agent Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="AgentHealthy")].status`
// +kubebuilder:printcolumn:name="Flow Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="TelemetryFlowHealthy")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type MetricPipeline struct {
	metav1.TypeMeta `json:",inline"`
	// +kubebuilder:validation:Optional
	metav1.ObjectMeta `json:"metadata"`

	// Spec defines the desired characteristics of MetricPipeline.
	// +kubebuilder:validation:Optional
	Spec MetricPipelineSpec `json:"spec"`

	// Status represents the current information/status of MetricPipeline.
	// +kubebuilder:validation:Optional
	Status MetricPipelineStatus `json:"status"`
}

// MetricPipelineSpec defines the desired state of MetricPipeline.
type MetricPipelineSpec struct {
	// Input configures additional inputs for metric collection.
	// +kubebuilder:validation:Optional
	Input MetricPipelineInput `json:"input"`

	// Output configures the backend to which metrics are sent. You must specify exactly one output per pipeline.
	// +kubebuilder:validation:Required
	Output MetricPipelineOutput `json:"output"`

	// Transforms specify a list of transformations to apply to telemetry data.
	// +kubebuilder:validation:Optional
	Transforms []TransformSpec `json:"transform,omitempty"`

	// Filter specifies a list of filters to apply to telemetry data.
	// +kubebuilder:validation:Optional
	Filters []FilterSpec `json:"filter,omitempty"`
}

// MetricPipelineInput configures additional inputs for metric collection.
type MetricPipelineInput struct {
	// Prometheus input configures collection of application metrics in the pull-based Prometheus protocol using endpoint discovery based on annotations.
	// +kubebuilder:validation:Optional
	Prometheus *MetricPipelinePrometheusInput `json:"prometheus,omitempty"`
	// Runtime input configures collection of Kubernetes runtime metrics.
	// +kubebuilder:validation:Optional
	Runtime *MetricPipelineRuntimeInput `json:"runtime,omitempty"`
	// Istio input configures collection of Istio metrics from applications running in the Istio service mesh.
	// +kubebuilder:validation:Optional
	Istio *MetricPipelineIstioInput `json:"istio,omitempty"`
	// OTLP input configures the push endpoint to receive metrics from an OTLP source.
	// +kubebuilder:validation:Optional
	OTLP *OTLPInput `json:"otlp,omitempty"`
}

// MetricPipelinePrometheusInput collection of application metrics in the pull-based Prometheus protocol using endpoint discovery based on annotations.
type MetricPipelinePrometheusInput struct {
	// Enabled specifies if the 'prometheus' input is enabled. If enabled, Service endpoints and Pods marked with `prometheus.io/scrape=true` annotation are scraped. The default is `false`.
	// +kubebuilder:validation:Optional
	Enabled *bool `json:"enabled,omitempty"`
	// Namespaces specifies from which namespaces metrics are collected. By default, all namespaces except the system namespaces are enabled. To enable all namespaces including system namespaces, use an empty struct notation.
	// +kubebuilder:validation:Optional
	Namespaces *NamespaceSelector `json:"namespaces,omitempty"`
	// DiagnosticMetrics configures collection of additional diagnostic metrics. The default is `false`.
	// +kubebuilder:validation:Optional
	DiagnosticMetrics *MetricPipelineIstioInputDiagnosticMetrics `json:"diagnosticMetrics,omitempty"`
}

// MetricPipelineRuntimeInput configures collection of Kubernetes runtime metrics.
type MetricPipelineRuntimeInput struct {
	// Enabled specifies if the 'runtime' input is enabled. If enabled, runtime metrics are collected. The default is `false`.
	// +kubebuilder:validation:Optional
	Enabled *bool `json:"enabled,omitempty"`
	// Namespaces specifies from which namespaces metrics are collected. By default, all namespaces except the system namespaces are enabled. To enable all namespaces including system namespaces, use an empty struct notation.
	// +kubebuilder:validation:Optional
	Namespaces *NamespaceSelector `json:"namespaces,omitempty"`
	// Resources configures the Kubernetes resource types for which metrics are collected.
	// +kubebuilder:validation:Optional
	Resources *MetricPipelineRuntimeInputResources `json:"resources,omitempty"`
}

// MetricPipelineRuntimeInputResources configures the Kubernetes resource types for which metrics are collected.
type MetricPipelineRuntimeInputResources struct {
	// Pod configures Pod runtime metrics collection.
	// +kubebuilder:validation:Optional
	Pod *MetricPipelineRuntimeInputResource `json:"pod,omitempty"`
	// Container configures container runtime metrics collection.
	// +kubebuilder:validation:Optional
	Container *MetricPipelineRuntimeInputResource `json:"container,omitempty"`
	// Node configures Node runtime metrics collection.
	// +kubebuilder:validation:Optional
	Node *MetricPipelineRuntimeInputResource `json:"node,omitempty"`
	// Volume configures Volume runtime metrics collection.
	// +kubebuilder:validation:Optional
	Volume *MetricPipelineRuntimeInputResource `json:"volume,omitempty"`
	// DaemonSet configures DaemonSet runtime metrics collection.
	// +kubebuilder:validation:Optional
	DaemonSet *MetricPipelineRuntimeInputResource `json:"daemonset,omitempty"`
	// Deployment configures Deployment runtime metrics collection.
	// +kubebuilder:validation:Optional
	Deployment *MetricPipelineRuntimeInputResource `json:"deployment,omitempty"`
	// StatefulSet configures StatefulSet runtime metrics collection.
	// +kubebuilder:validation:Optional
	StatefulSet *MetricPipelineRuntimeInputResource `json:"statefulset,omitempty"`
	// Job configures Job runtime metrics collection.
	// +kubebuilder:validation:Optional
	Job *MetricPipelineRuntimeInputResource `json:"job,omitempty"`
}

// MetricPipelineRuntimeInputResource configures if the collection of runtime metrics is enabled for a specific resource type. The collection is enabled by default.
type MetricPipelineRuntimeInputResource struct {
	// Enabled specifies that the runtime metrics for the resource type are collected. The default is `true`.
	// +kubebuilder:validation:Optional
	Enabled *bool `json:"enabled,omitempty"`
}

// MetricPipelineIstioInput defines the Istio scraping section.
type MetricPipelineIstioInput struct {
	// Enabled specifies if the 'istio' input is enabled. If enabled, istio-proxy metrics are scraped from Pods that have the istio-proxy sidecar injected. The default is `false`.
	// +kubebuilder:validation:Optional
	Enabled *bool `json:"enabled,omitempty"`
	// Namespaces configures the namespaces for which the collection should be activated. By default, all namespaces excluding system namespaces are enabled. To enable all namespaces including system namespaces, use an empty struct notation.
	// +kubebuilder:validation:Optional
	Namespaces *NamespaceSelector `json:"namespaces,omitempty"`
	// DiagnosticMetrics configures collection of additional diagnostic metrics. The default is `false`.
	// +kubebuilder:validation:Optional
	DiagnosticMetrics *MetricPipelineIstioInputDiagnosticMetrics `json:"diagnosticMetrics,omitempty"`
	// EnvoyMetrics enables the collection of additional Envoy metrics with prefix `envoy_`. The default is `false`.
	// +kubebuilder:validation:Optional
	EnvoyMetrics *EnvoyMetrics `json:"envoyMetrics,omitempty"`
}

// MetricPipelineIstioInputDiagnosticMetrics defines the diagnostic metrics configuration section
type MetricPipelineIstioInputDiagnosticMetrics struct {
	// If enabled, diagnostic metrics are collected. The default is `false`.
	// +kubebuilder:validation:Optional
	Enabled *bool `json:"enabled,omitempty"`
}

// MetricPipelineOutput defines the output configuration section.
type MetricPipelineOutput struct {
	// OTLP output defines an output using the OpenTelemetry protocol.
	// +kubebuilder:validation:Required
	OTLP *OTLPOutput `json:"otlp"`
}

// MetricPipelineStatus defines the observed state of MetricPipeline.
type MetricPipelineStatus struct {
	// An array of conditions describing the status of the pipeline.
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// EnvoyMetrics defines the configuration for scraping Envoy metrics.
type EnvoyMetrics struct {
	// Enabled specifies that Envoy metrics with prefix `envoy_` are scraped additionally. The default is `false`.
	// +kubebuilder:validation:Optional
	Enabled *bool `json:"enabled,omitempty"`
}
