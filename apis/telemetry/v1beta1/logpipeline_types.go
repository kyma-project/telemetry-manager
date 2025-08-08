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

type Mode int

const (
	OTel Mode = iota
	FluentBit
)

//nolint:gochecknoinits // SchemeBuilder's registration is required.
func init() {
	SchemeBuilder.Register(&LogPipeline{}, &LogPipelineList{})
}

// +kubebuilder:object:root=true
// LogPipelineList contains a list of LogPipeline
type LogPipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []LogPipeline `json:"items"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories={kyma-telemetry,kyma-telemetry-pipelines}
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Configuration Generated",type=string,JSONPath=`.status.conditions[?(@.type=="ConfigurationGenerated")].status`
// +kubebuilder:printcolumn:name="Gateway Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="GatewayHealthy")].status`
// +kubebuilder:printcolumn:name="Agent Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="AgentHealthy")].status`
// +kubebuilder:printcolumn:name="Flow Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="TelemetryFlowHealthy")].status`
// +kubebuilder:printcolumn:name="Unsupported Mode",type=boolean,JSONPath=`.status.unsupportedMode`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:storageversion
// LogPipeline is the Schema for the logpipelines API
type LogPipeline struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Defines the desired state of LogPipeline
	Spec LogPipelineSpec `json:"spec,omitempty"`
	// Shows the observed state of the LogPipeline
	Status LogPipelineStatus `json:"status,omitempty"`
}

// LogPipelineSpec defines the desired state of LogPipeline
// +kubebuilder:validation:XValidation:rule="!((has(self.output.http) || has(self.output.custom))  && has(self.input.otlp))", message="otlp input is only supported with otlp output"
// +kubebuilder:validation:XValidation:rule="!(has(self.output.otlp) && has(self.input.runtime.dropLabels))", message="input.runtime.dropLabels is not supported with otlp output"
// +kubebuilder:validation:XValidation:rule="!(has(self.output.otlp) && has(self.input.runtime.keepAnnotations))", message="input.runtime.keepAnnotations is not supported with otlp output"
// +kubebuilder:validation:XValidation:rule="!(has(self.output.otlp) && has(self.filters))", message="filters are not supported with otlp output"
// +kubebuilder:validation:XValidation:rule="!(has(self.output.otlp) && has(self.files))", message="files not supported with otlp output"
// +kubebuilder:validation:XValidation:rule="!(has(self.output.otlp) && has(self.variables))", message="variables not supported with otlp output"
type LogPipelineSpec struct {
	// Configures additional inputs for log collection.
	Input LogPipelineInput `json:"input,omitempty"`
	// Configures custom Fluent-Bit `filters` to transform logs. Only available when using an output of type `http` and `custom`.
	Filters []LogPipelineFilter `json:"filters,omitempty"`
	// Configures the output where the logs will be send to. Exactly one output must be specified.
	Output LogPipelineOutput `json:"output,omitempty"`
	// A list of content snippets that are mounted as files in the Fluent Bit configuration, which can be linked in the `custom` filters and a `custom` output. Only available when using an output of type `http` and `custom`.
	Files []LogPipelineFileMount `json:"files,omitempty"`
	// A list of mappings from Kubernetes Secret keys to environment variables. Mapped keys are mounted as environment variables, so that they are available as [Variables](https://docs.fluentbit.io/manual/administration/configuring-fluent-bit/classic-mode/variables) in the `custom` filters and a `custom` output. Only available when using an output of type `http` and `custom`.
	Variables []LogPipelineVariableRef `json:"variables,omitempty"`
}

// LogPipelineInput configures additional inputs for log collection.
type LogPipelineInput struct {
	// Configures the log collection from application containers stdout/stderr by tailing the log files of the underlying container runtime.
	Runtime *LogPipelineRuntimeInput `json:"runtime,omitempty"`
	// Configures the push endpoint to receive logs from a OTLP source.
	OTLP *OTLPInput `json:"otlp,omitempty"`
}

// LogPipelineRuntimeInput configures the log collection from application containers stdout/stderr by tailing the log files of the underlying container runtime.
type LogPipelineRuntimeInput struct {
	// If enabled, application logs are collected from application containers stdout/stderr. The default is `true`.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// Describes whether application logs from specific Namespaces are selected. The options are mutually exclusive. System Namespaces are excluded by default. Use the `system` attribute with value `true` to enable them.
	Namespaces LogPipelineNamespaceSelector `json:"namespaces,omitempty"`
	// Describes whether application logs from specific containers are selected. The options are mutually exclusive.
	Containers LogPipelineContainerSelector `json:"containers,omitempty"`
	// Defines whether to keep all Kubernetes annotations. The default is `false`.  Only available when using an output of type `http` and `custom`.
	// +optional
	KeepAnnotations *bool `json:"keepAnnotations,omitempty"`
	// Defines whether to drop all Kubernetes labels. The default is `false`. Only available when using an output of type `http` and `custom`. For an `otlp` output, use the label enrichement feature in the Telemetry resource instead.
	// +optional
	DropLabels *bool `json:"dropLabels,omitempty"`
	// If the log data is in JSON and it is successfully parsed, the original log data is retained if `KeepOriginalBody` is set to `true`. Otherwise, the original log data will be removed from the log record. The default is `true`.
	// +optional
	KeepOriginalBody *bool `json:"keepOriginalBody,omitempty"`
}

// LogPipelineNamespaceSelector describes whether application logs from specific Namespaces are selected. The options are mutually exclusive. System Namespaces are excluded by default. Use the `system` attribute with value `true` to enable them.
type LogPipelineNamespaceSelector struct {
	// Include only the container logs of the specified Namespace names.
	Include []string `json:"include,omitempty"`
	// Exclude the container logs of the specified Namespace names.
	Exclude []string `json:"exclude,omitempty"`
	// Set to `true` if collecting from all Namespaces must also include the system Namespaces like kube-system, istio-system, and kyma-system.
	System bool `json:"system,omitempty"`
}

// LogPipelineContainerSelector describes whether application logs from specific containers are selected. The options are mutually exclusive.
type LogPipelineContainerSelector struct {
	// Specifies to include only the container logs with the specified container names.
	Include []string `json:"include,omitempty"`
	// Specifies to exclude only the container logs with the specified container names.
	Exclude []string `json:"exclude,omitempty"`
}

// LogPipelineFilter configures custom Fluent-Bit `filters` to transform logs. Only available when using an output of type `http` and `custom`.
type LogPipelineFilter struct {
	// Defines a custom filter in the [Fluent Bit syntax](https://docs.fluentbit.io/manual/pipeline/outputs). Note: If you use a `custom` filter, you put the LogPipeline in unsupported mode. Only available when using an output of type `http` and `custom`.
	Custom string `json:"custom,omitempty"`
}

// LogPipelineOutput configures the output where the logs will be send to. Exactly one output must be specified.
// +kubebuilder:validation:XValidation:rule="has(self.otlp) == has(oldSelf.otlp)", message="Switching to or away from OTLP output is not supported. Please re-create the LogPipeline instead"
// +kubebuilder:validation:XValidation:rule="(!has(self.custom) && !has(self.http)) || !(has(self.custom) && has(self.http))", message="Exactly one output must be defined"
// +kubebuilder:validation:XValidation:rule="(!has(self.custom) && !has(self.otlp)) || ! (has(self.custom) && has(self.otlp))", message="Exactly one output must be defined"
// +kubebuilder:validation:XValidation:rule="(!has(self.http) && !has(self.otlp)) || ! (has(self.http) && has(self.otlp))", message="Exactly one output must be defined"
type LogPipelineOutput struct {
	// Defines a custom output in the [Fluent Bit syntax](https://docs.fluentbit.io/manual/pipeline/outputs) where you want to push the logs. Note: If you use a `custom` output, you put the LogPipeline in unsupported mode. Only available when using an output of type `http` and `custom`.
	Custom string `json:"custom,omitempty"`
	// Configures an HTTP-based output compatible with the Fluent Bit HTTP output plugin.
	HTTP *LogPipelineHTTPOutput `json:"http,omitempty"`
	// Defines an output using the OpenTelemetry protocol.
	OTLP *OTLPOutput `json:"otlp,omitempty"`
}

// LogPipelineHTTPOutput configures an HTTP-based output compatible with the Fluent Bit HTTP output plugin.
type LogPipelineHTTPOutput struct {
	// Defines the host of the HTTP backend.
	Host ValueType `json:"host,omitempty"`
	// Defines the basic auth user.
	User ValueType `json:"user,omitempty"`
	// Defines the basic auth password.
	Password ValueType `json:"password,omitempty"`
	// Defines the URI of the HTTP backend. Default is "/".
	URI string `json:"uri,omitempty"`
	// Defines the port of the HTTP backend. Default is 443.
	Port string `json:"port,omitempty"`
	// Defines the compression algorithm to use.
	Compress string `json:"compress,omitempty"`
	// Data format to be used in the HTTP request body. Default is `json`.
	Format string `json:"format,omitempty"`
	// Configures TLS for the HTTP backend.
	TLSConfig OutputTLS `json:"tls,omitempty"`
	// Enables de-dotting of Kubernetes labels and annotations for compatibility with ElasticSearch based backends. Dots (.) will be replaced by underscores (_). Default is `false`.
	Dedot bool `json:"dedot,omitempty"`
}

// Provides file content to be consumed by a LogPipeline configuration
type LogPipelineFileMount struct {
	Name    string `json:"name,omitempty"`
	Content string `json:"content,omitempty"`
}

// References a Kubernetes secret that should be provided as environment variable to Fluent Bit
type LogPipelineVariableRef struct {
	// Name of the variable to map.
	Name      string          `json:"name,omitempty"`
	ValueFrom ValueFromSource `json:"valueFrom,omitempty"`
}

// LogPipelineStatus shows the observed state of the LogPipeline
type LogPipelineStatus struct {
	// An array of conditions describing the status of the pipeline.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Is active when the LogPipeline uses a `custom` output or filter; see [unsupported mode](https://github.com/kyma-project/telemetry-manager/blob/main/docs/user/02-logs.md#unsupported-mode).
	UnsupportedMode *bool `json:"unsupportedMode,omitempty"`
}
