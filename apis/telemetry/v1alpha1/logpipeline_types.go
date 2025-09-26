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
	SchemeBuilder.Register(&LogPipeline{}, &LogPipelineList{})
}

// LogPipelineList contains a list of LogPipeline
// +kubebuilder:object:root=true
type LogPipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []LogPipeline `json:"items"`
}

// LogPipeline is the Schema for the logpipelines API
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories={kyma-telemetry,kyma-telemetry-pipelines}
// +kubebuilder:metadata:labels={app.kubernetes.io/component=controller,app.kubernetes.io/managed-by=kyma,app.kubernetes.io/name=telemetry-manager,app.kubernetes.io/part-of=telemetry,kyma-project.io/module=telemetry}
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Configuration Generated",type=string,JSONPath=`.status.conditions[?(@.type=="ConfigurationGenerated")].status`
// +kubebuilder:printcolumn:name="Agent Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="AgentHealthy")].status`
// +kubebuilder:printcolumn:name="Flow Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="TelemetryFlowHealthy")].status`
// +kubebuilder:printcolumn:name="Unsupported Mode",type=boolean,JSONPath=`.status.unsupportedMode`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type LogPipeline struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Defines the desired state of LogPipeline
	Spec LogPipelineSpec `json:"spec,omitempty"`
	// Shows the observed state of the LogPipeline
	Status LogPipelineStatus `json:"status,omitempty"`
}

// LogPipelineSpec defines the desired state of LogPipeline
// +kubebuilder:validation:XValidation:rule="!(has(self.output.otlp) && has(self.input.application.dropLabels))", message="input.application.dropLabels is not supported with otlp output"
// +kubebuilder:validation:XValidation:rule="!(has(self.output.otlp) && has(self.input.application.keepAnnotations))", message="input.application.keepAnnotations is not supported with otlp output"
// +kubebuilder:validation:XValidation:rule="!(has(self.output.otlp) && has(self.filters))", message="filters are not supported with otlp output"
// +kubebuilder:validation:XValidation:rule="!(has(self.output.otlp) && has(self.files))", message="files not supported with otlp output"
// +kubebuilder:validation:XValidation:rule="!(has(self.output.otlp) && has(self.variables))", message="variables not supported with otlp output"
// +kubebuilder:validation:XValidation:rule="has(self.output.otlp) || !(has(self.transform))", message="transform is only supported with otlp output"
// +kubebuilder:validation:XValidation:rule="has(self.output.otlp) || !(has(self.input.otlp))", message="otlp input is only supported with otlp output"
type LogPipelineSpec struct {
	// Input configures additional inputs for log collection.
	Input LogPipelineInput `json:"input,omitempty"`
	// Filters configures custom Fluent Bit `filters` to transform logs. Only available when using an output of type `http` and `custom`.
	Filters []LogPipelineFilter `json:"filters,omitempty"`
	// Output configures the backend to which logs are sent. You must specify exactly one output per pipeline.
	// +kubebuilder:validation:Required
	Output LogPipelineOutput `json:"output,omitempty"`
	// Files is a list of content snippets that are mounted as files in the Fluent Bit configuration, which can be linked in the `custom` filters and a `custom` output. Only available when using an output of type `http` and `custom`.
	Files []LogPipelineFileMount `json:"files,omitempty"`
	// Variables is a list of mappings from Kubernetes Secret keys to environment variables. Mapped keys are mounted as environment variables, so that they are available as [Variables](https://docs.fluentbit.io/manual/administration/configuring-fluent-bit/classic-mode/variables) in the `custom` filters and a `custom` output. Only available when using an output of type `http` and `custom`.
	Variables []LogPipelineVariableRef `json:"variables,omitempty"`
	// Transforms specify a list of transformations to apply to telemetry data.
	// +optional
	Transforms []TransformSpec `json:"transform,omitempty"`
}

// LogPipelineInput configures additional inputs for log collection.
type LogPipelineInput struct {
	// Application input configures the log collection from application containers stdout/stderr by tailing the log files of the underlying container runtime.
	Application *LogPipelineApplicationInput `json:"application,omitempty"`
	// OTLP input configures the push endpoint to receive logs from a OTLP source.
	OTLP *OTLPInput `json:"otlp,omitempty"`
}

// LogPipelineApplicationInput configures the log collection from application containers stdout/stderr by tailing the log files of the underlying container runtime.
type LogPipelineApplicationInput struct {
	// If enabled, application logs are collected from application containers stdout/stderr. The default is `true`.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// Namespaces describes whether application logs from specific namespaces are selected. The options are mutually exclusive. System namespaces are excluded by default. Use the `system` attribute with value `true` to enable them.
	Namespaces LogPipelineNamespaceSelector `json:"namespaces,omitempty"`
	// Containers describes whether application logs from specific containers are selected. The options are mutually exclusive.
	Containers LogPipelineContainerSelector `json:"containers,omitempty"`
	// KeepAnnotations defines whether to keep all Kubernetes annotations. The default is `false`.  Only available when using an output of type `http` and `custom`.
	// +optional
	KeepAnnotations *bool `json:"keepAnnotations,omitempty"`
	// DropLabels defines whether to drop all Kubernetes labels. The default is `false`. Only available when using an output of type `http` and `custom`. For an `otlp` output, use the label enrichement feature in the Telemetry resource instead.
	// +optional
	DropLabels *bool `json:"dropLabels,omitempty"`
	// KeepOriginalBody retains the original log data if the log data is in JSON and it is successfully parsed. If set to `false`, the original log data is removed from the log record. The default is `true`.
	// +optional
	KeepOriginalBody *bool `json:"keepOriginalBody,omitempty"`
}

// LogPipelineNamespaceSelector describes whether application logs from specific Namespaces are selected. The options are mutually exclusive. System Namespaces are excluded by default. Use the `system` attribute with value `true` to enable them.
// +kubebuilder:validation:MaxProperties=1
type LogPipelineNamespaceSelector struct {
	// Include only the container logs of the specified Namespace names.
	Include []string `json:"include,omitempty"`
	// Exclude the container logs of the specified Namespace names.
	Exclude []string `json:"exclude,omitempty"`
	// System specifies whether to collect logs from system namespaces. If set to `true`, you collect logs from all namespaces including system namespaces, such as like kube-system, istio-system, and kyma-system. The default is `false`.
	System bool `json:"system,omitempty"`
}

// LogPipelineContainerSelector describes whether application logs from specific containers are selected. The options are mutually exclusive.
// +kubebuilder:validation:MaxProperties=1
type LogPipelineContainerSelector struct {
	// Include specifies to include only the container logs with the specified container names.
	Include []string `json:"include,omitempty"`
	// Exclude specifies to exclude only the container logs with the specified container names.
	Exclude []string `json:"exclude,omitempty"`
}

// LogPipelineFilter configures custom Fluent Bit `filters` to transform logs. Only available when using an output of type `http` and `custom`.
type LogPipelineFilter struct {
	// Custom defines a custom filter in the [Fluent Bit syntax](https://docs.fluentbit.io/manual/pipeline/outputs). If you use a `custom` filter, you put the LogPipeline in unsupported mode. Only available when using an output of type `http` and `custom`.
	Custom string `json:"custom,omitempty"`
}

// LogPipelineOutput configures the backend to which logs are sent. You must specify exactly one output per pipeline.
// +kubebuilder:validation:XValidation:rule="has(self.otlp) == has(oldSelf.otlp)", message="Switching to or away from OTLP output is not supported. Please re-create the LogPipeline instead"
// +kubebuilder:validation:MaxProperties=1
// +kubebuilder:validation:MinProperties=1
type LogPipelineOutput struct {
	// Custom defines a custom output in the [Fluent Bit syntax](https://docs.fluentbit.io/manual/pipeline/outputs) where you want to push the logs. If you use a `custom` output, you put the LogPipeline in unsupported mode. Only available when using an output of type `http` and `custom`.
	Custom string `json:"custom,omitempty"`
	// HTTP configures an HTTP-based output compatible with the Fluent Bit HTTP output plugin.
	HTTP *LogPipelineHTTPOutput `json:"http,omitempty"`
	// OTLP defines an output using the OpenTelemetry protocol.
	OTLP *OTLPOutput `json:"otlp,omitempty"`
}

// LogPipelineHTTPOutput configures an HTTP-based output compatible with the Fluent Bit HTTP output plugin.
type LogPipelineHTTPOutput struct {
	// Host defines the host of the HTTP backend.
	// +kubebuilder:validation:Required
	Host ValueType `json:"host"`
	// User defines the basic auth user.
	User *ValueType `json:"user,omitempty"`
	// Password defines the basic auth password.
	Password *ValueType `json:"password,omitempty"`
	// URI defines the URI of the HTTP backend. Default is "/".
	// +kubebuilder:validation:Pattern=`^/.*$`
	URI string `json:"uri,omitempty"`
	// Port defines the port of the HTTP backend. Default is 443.
	Port string `json:"port,omitempty"`
	// Compress defines the compression algorithm to use. Either `none` or `gzip`. Default is `none`.
	Compress string `json:"compress,omitempty"`
	// Format is the data format to be used in the HTTP request body. Either `gelf`, `json`, `json_stream`, `json_lines`, or `msgpack`. Default is `json`.
	Format string `json:"format,omitempty"`
	// TLS configures TLS for the HTTP backend.
	TLS LogPipelineOutputTLS `json:"tls,omitempty"`
	// Dedot enables de-dotting of Kubernetes labels and annotations. For compatibility with OpenSearch-based backends, dots (.) are replaced by underscores (_). Default is `false`.
	Dedot bool `json:"dedot,omitempty"`
}

// LogPipelineOutputTLS configures TLS for the HTTP backend.
// +kubebuilder:validation:XValidation:rule="has(self.cert) == has(self.key)", message="Can define either both 'cert' and 'key', or neither"
type LogPipelineOutputTLS struct {
	// Disabled specifies if TLS is disabled or enabled. Default is `false`.
	Disabled bool `json:"disabled,omitempty"`
	// If `true`, the validation of certificates is skipped. Default is `false`.
	SkipCertificateValidation bool `json:"skipCertificateValidation,omitempty"`
	// CA defines an optional CA certificate for server certificate verification when using TLS. The certificate must be provided in PEM format.
	CA *ValueType `json:"ca,omitempty"`
	// Cert defines a client certificate to use when using TLS. The certificate must be provided in PEM format.
	Cert *ValueType `json:"cert,omitempty"`
	// Key defines the client key to use when using TLS. The key must be provided in PEM format.
	Key *ValueType `json:"key,omitempty"`
}

// LogPipelineFileMount provides file content to be consumed by a LogPipeline configuration
type LogPipelineFileMount struct {
	// Name of the file under which the content is mounted in the Fluent Bit configuration.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Content of the file to be mounted in the Fluent Bit configuration.
	// +kubebuilder:validation:Required
	Content string `json:"content"`
}

// LogPipelineVariableRef references a Kubernetes secret that should be provided as environment variable to Fluent Bit
type LogPipelineVariableRef struct {
	// Name of the variable to map.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// ValueFrom specifies the secret and key to select the value to map.
	// +kubebuilder:validation:Required
	ValueFrom ValueFromSource `json:"valueFrom"`
}

// LogPipelineStatus shows the observed state of the LogPipeline
type LogPipelineStatus struct {
	// An array of conditions describing the status of the pipeline.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Is active when the LogPipeline uses a `custom` output or filter; see [unsupported mode](https://github.com/kyma-project/telemetry-manager/blob/main/docs/user/02-logs.md#unsupported-mode).
	UnsupportedMode *bool `json:"unsupportedMode,omitempty"`
}
