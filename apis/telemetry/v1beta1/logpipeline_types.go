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
// +kubebuilder:printcolumn:name="Gateway Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="GatewayHealthy")].status`
// +kubebuilder:printcolumn:name="Agent Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="AgentHealthy")].status`
// +kubebuilder:printcolumn:name="Flow Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="TelemetryFlowHealthy")].status`
// +kubebuilder:printcolumn:name="Unsupported Mode",type=boolean,JSONPath=`.status.unsupportedMode`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:storageversion
type LogPipeline struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Defines the desired state of LogPipeline
	// +kubebuilder:validation:Optional
	Spec LogPipelineSpec `json:"spec,omitempty"`
	// Shows the observed state of the LogPipeline
	// +kubebuilder:validation:Optional
	Status LogPipelineStatus `json:"status,omitempty"`
}

// LogPipelineSpec defines the desired state of LogPipeline
// +kubebuilder:validation:XValidation:rule="!(has(self.output.otlp) && has(self.input.runtime.dropLabels))", message="input.runtime.dropLabels is not supported with otlp output"
// +kubebuilder:validation:XValidation:rule="!(has(self.output.otlp) && has(self.input.runtime.keepAnnotations))", message="input.runtime.keepAnnotations is not supported with otlp output"
// +kubebuilder:validation:XValidation:rule="!(has(self.output.otlp) && has(self.filters))", message="filters are not supported with otlp output"
// +kubebuilder:validation:XValidation:rule="!(has(self.output.otlp) && has(self.files))", message="files not supported with otlp output"
// +kubebuilder:validation:XValidation:rule="!(has(self.output.otlp) && has(self.variables))", message="variables not supported with otlp output"
// +kubebuilder:validation:XValidation:rule="has(self.output.otlp) || !(has(self.transform))", message="transform is only supported with otlp output"
// +kubebuilder:validation:XValidation:rule="has(self.output.otlp) || !(has(self.filter))", message="filter is only supported with otlp output"
// +kubebuilder:validation:XValidation:rule="has(self.output.otlp) || !(has(self.input.otlp))", message="otlp input is only supported with otlp output"
type LogPipelineSpec struct {
	// Input configures additional inputs for log collection.
	// +kubebuilder:validation:Optional
	Input LogPipelineInput `json:"input,omitempty"`
	// FluentBitFilters configures custom Fluent Bit `filters` to transform logs. Only available when using an output of type `http` and `custom`.
	// +kubebuilder:validation:Optional
	FluentBitFilters []LogPipelineFilter `json:"filters,omitempty"`
	// Output configures the backend to which logs are sent. You must specify exactly one output per pipeline.
	// +kubebuilder:validation:Required
	Output LogPipelineOutput `json:"output"`
	// Files is a list of content snippets that are mounted as files in the Fluent Bit configuration, which can be linked in the `custom` filters and a `custom` output. Only available when using an output of type `http` and `custom`.
	// +kubebuilder:validation:Optional
	Files []LogPipelineFileMount `json:"files,omitempty"`
	// Variables is a list of mappings from Kubernetes Secret keys to environment variables. Mapped keys are mounted as environment variables, so that they are available as [Variables](https://docs.fluentbit.io/manual/administration/configuring-fluent-bit/classic-mode/variables) in the `custom` filters and a `custom` output. Only available when using an output of type `http` and `custom`.
	// +kubebuilder:validation:Optional
	Variables []LogPipelineVariableRef `json:"variables,omitempty"`
	// Transforms specify a list of transformations to apply to telemetry data.
	// +kubebuilder:validation:Optional
	Transforms []TransformSpec `json:"transform,omitempty"`
	// Filters specifies a list of filters to apply to telemetry data.
	// +kubebuilder:validation:Optional
	Filters []FilterSpec `json:"filter,omitempty"`
}

// LogPipelineInput configures additional inputs for log collection.
type LogPipelineInput struct {
	// Runtime input configures the log collection from application containers stdout/stderr by tailing the log files of the underlying container runtime.
	// +kubebuilder:validation:Optional
	Runtime *LogPipelineRuntimeInput `json:"runtime,omitempty"`
	// OTLP input configures the push endpoint to receive logs from a OTLP source.
	// +kubebuilder:validation:Optional
	OTLP *OTLPInput `json:"otlp,omitempty"`
}

// LogPipelineRuntimeInput configures the log collection from application containers stdout/stderr by tailing the log files of the underlying container runtime.
type LogPipelineRuntimeInput struct {
	// Enabled specifies if the 'runtime' input is enabled. If enabled, application logs are collected from application containers stdout/stderr. The default is `true`.
	// +kubebuilder:validation:Optional
	Enabled *bool `json:"enabled,omitempty"`
	// Namespaces describes whether application logs from specific namespaces are selected. The options are mutually exclusive. System namespaces are excluded by default. Use the `system` attribute with value `true` to enable them.
	// +kubebuilder:validation:Optional
	Namespaces LogPipelineNamespaceSelector `json:"namespaces,omitempty"`
	// Containers describes whether application logs from specific containers are selected. The options are mutually exclusive.
	// +kubebuilder:validation:Optional
	Containers LogPipelineContainerSelector `json:"containers,omitempty"`
	// KeepAnnotations defines whether to keep all Kubernetes annotations. The default is `false`.  Only available when using an output of type `http` and `custom`.
	// +kubebuilder:validation:Optional
	KeepAnnotations *bool `json:"keepAnnotations,omitempty"`
	// DropLabels defines whether to drop all Kubernetes labels. The default is `false`. Only available when using an output of type `http` and `custom`. For an `otlp` output, use the label enrichement feature in the Telemetry resource instead.
	// +kubebuilder:validation:Optional
	DropLabels *bool `json:"dropLabels,omitempty"`
	// KeepOriginalBody retains the original log data if the log data is in JSON and it is successfully parsed. If set to `false`, the original log data is removed from the log record. The default is `true`.
	// +kubebuilder:validation:Optional
	KeepOriginalBody *bool `json:"keepOriginalBody,omitempty"`
}

// LogPipelineNamespaceSelector describes whether application logs from specific Namespaces are selected. The options are mutually exclusive. System Namespaces are excluded by default. Use the `system` attribute with value `true` to enable them.
// +kubebuilder:validation:XValidation:rule="(has(self.include) == true ? 1 : 0) + (has(self.exclude) == true ? 1 : 0) + (has(self.system) == true ? 1 : 0) <= 1",message="Only one of 'include', 'exclude' or 'system' can be defined"
type LogPipelineNamespaceSelector struct {
	// Include specifies the list of namespace names to include when collecting container logs. By default, logs from all namespaces are collected, except system namespaces. You cannot specify an include list together with an exclude list.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:items:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:items:MaxLength=63
	Include []string `json:"include,omitempty"`
	// Exclude specifies the list of namespace names to exclude when collecting container logs. By default, logs from all namespaces are collected, except system namespaces. You cannot specify an exclude list together with an include list.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:items:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:items:MaxLength=63
	Exclude []string `json:"exclude,omitempty"`
	// System specifies whether to collect logs from system namespaces. If set to `true`, you collect logs from all namespaces including system namespaces, such as like kube-system, istio-system, and kyma-system. The default is `false`.
	// +kubebuilder:validation:Optional
	System bool `json:"system,omitempty"`
}

// LogPipelineContainerSelector describes whether application logs from specific containers are selected. The options are mutually exclusive.
// +kubebuilder:validation:XValidation:rule="!(has(self.include) && has(self.exclude))",message="Only one of 'include' or 'exclude' can be defined"
type LogPipelineContainerSelector struct {
	// Include specifies to include only the container logs with the specified container names.
	// +kubebuilder:validation:Optional
	Include []string `json:"include,omitempty"`
	// Exclude specifies to exclude only the container logs with the specified container names.
	// +kubebuilder:validation:Optional
	Exclude []string `json:"exclude,omitempty"`
}

// LogPipelineFilter configures custom Fluent Bit `filters` to transform logs. Only available when using an output of type `http` and `custom`.
type LogPipelineFilter struct {
	// Custom defines a custom filter in the [Fluent Bit syntax](https://docs.fluentbit.io/manual/pipeline/outputs). If you use a `custom` filter, you put the LogPipeline in unsupported mode. Only available when using an output of type `http` and `custom`.
	// +kubebuilder:validation:Optional
	Custom string `json:"custom,omitempty"`
}

// LogPipelineOutput configures the backend to which logs are sent. You must specify exactly one output per pipeline.
// +kubebuilder:validation:XValidation:rule="has(self.otlp) == has(oldSelf.otlp)", message="Switching to or away from OTLP output is not supported. Please re-create the LogPipeline instead"
// +kubebuilder:validation:XValidation:rule="(has(self.custom) == true ? 1 : 0) + (has(self.http) == true ? 1 : 0) + (has(self.otlp) == true ? 1 : 0) == 1",message="Exactly one output out of 'custom', 'http' or 'otlp' must be defined"
type LogPipelineOutput struct {
	// Custom defines a custom output in the [Fluent Bit syntax](https://docs.fluentbit.io/manual/pipeline/outputs) where you want to push the logs. If you use a `custom` output, you put the LogPipeline in unsupported mode. Only available when using an output of type `http` and `custom`.
	// +kubebuilder:validation:Optional
	Custom string `json:"custom,omitempty"`
	// HTTP configures an HTTP-based output compatible with the Fluent Bit HTTP output plugin.
	// +kubebuilder:validation:Optional
	HTTP *LogPipelineHTTPOutput `json:"http,omitempty"`
	// OTLP defines an output using the OpenTelemetry protocol.
	// +kubebuilder:validation:Optional
	OTLP *OTLPOutput `json:"otlp,omitempty"`
}

// LogPipelineHTTPOutput configures an HTTP-based output compatible with the Fluent Bit HTTP output plugin.
type LogPipelineHTTPOutput struct {
	// Host defines the host of the HTTP backend.
	// +kubebuilder:validation:Required
	Host ValueType `json:"host"`
	// User defines the basic auth user.
	// +kubebuilder:validation:Optional
	User *ValueType `json:"user,omitempty"`
	// Password defines the basic auth password.
	// +kubebuilder:validation:Optional
	Password *ValueType `json:"password,omitempty"`
	// URI defines the URI of the HTTP backend. Default is "/".
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^/.*$`
	URI string `json:"uri,omitempty"`
	// Port defines the port of the HTTP backend. Default is 443.
	// +kubebuilder:validation:Optional
	Port string `json:"port,omitempty"`
	// Compress defines the compression algorithm to use. Either `none` or `gzip`. Default is `none`.
	// +kubebuilder:validation:Optional
	Compress string `json:"compress,omitempty"`
	// Format is the data format to be used in the HTTP request body. Either `gelf`, `json`, `json_stream`, `json_lines`, or `msgpack`. Default is `json`.
	// +kubebuilder:validation:Optional
	Format string `json:"format,omitempty"`
	// TLS configures TLS for the HTTP backend.
	// +kubebuilder:validation:Optional
	TLSConfig OutputTLS `json:"tls,omitempty"`
	// Dedot enables de-dotting of Kubernetes labels and annotations. For compatibility with OpenSearch-based backends, dots (.) are replaced by underscores (_). Default is `false`.
	// +kubebuilder:validation:Optional
	Dedot bool `json:"dedot,omitempty"`
}

// LogPipelineFileMount provides file content to be consumed by a LogPipeline configuration
type LogPipelineFileMount struct {
	// Name of the file under which the content is mounted in the Fluent Bit configuration.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Content of the file to be mounted in the Fluent Bit configuration.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Content string `json:"content"`
}

// LogPipelineVariableRef references a Kubernetes secret that should be provided as environment variable to Fluent Bit
type LogPipelineVariableRef struct {
	// Name of the variable to map.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// ValueFrom specifies the secret and key to select the value to map.
	// +kubebuilder:validation:Required
	ValueFrom ValueFromSource `json:"valueFrom"`
}

// LogPipelineStatus shows the observed state of the LogPipeline
type LogPipelineStatus struct {
	// An array of conditions describing the status of the pipeline.
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Is active when the LogPipeline uses a `custom` output or filter; see [unsupported mode](https://github.com/kyma-project/telemetry-manager/blob/main/docs/user/02-logs.md#unsupported-mode).
	// +kubebuilder:validation:Optional
	UnsupportedMode *bool `json:"unsupportedMode,omitempty"`
}
