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
	metav1.ListMeta `json:"metadata"`

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
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:storageversion
type LogPipeline struct {
	metav1.TypeMeta `json:",inline"`
	// +kubebuilder:validation:Optional
	metav1.ObjectMeta `json:"metadata"`

	// Defines the desired state of LogPipeline
	// +kubebuilder:validation:Optional
	Spec LogPipelineSpec `json:"spec"`
	// Shows the observed state of the LogPipeline
	// +kubebuilder:validation:Optional
	Status LogPipelineStatus `json:"status"`
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
	Input LogPipelineInput `json:"input"`
	// Deprecated: The field is based on the Fluent Bit-based technology stack. Use the OpenTelemetry-based stack instead, see https://kyma-project.io/external-content/telemetry-manager/docs/user/integrate-otlp-backend/migration-to-otlp-logs.html.
	// FluentBitFilters configures custom Fluent Bit `filters` to transform logs. Only available when using an output of type `http` and `custom`.
	// +kubebuilder:validation:Optional
	FluentBitFilters []FluentBitFilter `json:"filters,omitempty"`
	// Output configures the backend to which logs are sent. You must specify exactly one output per pipeline.
	// +kubebuilder:validation:Required
	Output LogPipelineOutput `json:"output"`
	// Deprecated: The field is based on the Fluent Bit-based technology stack. Use the OpenTelemetry-based stack instead, see https://kyma-project.io/external-content/telemetry-manager/docs/user/integrate-otlp-backend/migration-to-otlp-logs.html.
	// FluentBitFiles is a list of content snippets that are mounted as files in the Fluent Bit configuration, which can be linked in the `custom` filters and a `custom` output. Only available when using an output of type `http` and `custom`.
	// +kubebuilder:validation:Optional
	FluentBitFiles []FluentBitFile `json:"files,omitempty"`
	// Deprecated: The field is based on the Fluent Bit-based technology stack. Use the OpenTelemetry-based stack instead, see https://kyma-project.io/external-content/telemetry-manager/docs/user/integrate-otlp-backend/migration-to-otlp-logs.html.
	// FluentBitVariables is a list of mappings from Kubernetes Secret keys to environment variables. Mapped keys are mounted as environment variables, so that they are available as [FluentBitVariables](https://docs.fluentbit.io/manual/administration/configuring-fluent-bit/classic-mode/variables) in the `custom` filters and a `custom` output. Only available when using an output of type `http` and `custom`.
	// +kubebuilder:validation:Optional
	FluentBitVariables []FluentBitVariable `json:"variables,omitempty"`
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
	// OTLP input configures the push endpoint to receive logs from an OTLP source.
	// +kubebuilder:validation:Optional
	OTLP *OTLPInput `json:"otlp,omitempty"`
}

// LogPipelineRuntimeInput configures the log collection from application containers stdout/stderr by tailing the log files of the underlying container runtime.
type LogPipelineRuntimeInput struct {
	// Enabled specifies if the 'runtime' input is enabled. If enabled, application logs are collected from application containers stdout/stderr. The default is `true`.
	// +kubebuilder:validation:Optional
	Enabled *bool `json:"enabled,omitempty"`
	// Namespaces describes whether application logs from specific namespaces are selected. The options are mutually exclusive. By default, all namespaces except the system namespaces are enabled. To enable all namespaces including system namespaces, use an empty struct notation.
	// +kubebuilder:validation:Optional
	Namespaces *NamespaceSelector `json:"namespaces,omitempty"`
	// Containers describes whether application logs from specific containers are selected. The options are mutually exclusive.
	// +kubebuilder:validation:Optional
	Containers *LogPipelineContainerSelector `json:"containers,omitempty"`
	// Deprecated: The field is based on the Fluent Bit-based technology stack. Use the OpenTelemetry-based stack instead, see https://kyma-project.io/external-content/telemetry-manager/docs/user/integrate-otlp-backend/migration-to-otlp-logs.html.
	// FluentBitKeepAnnotations defines whether to keep all Kubernetes annotations. The default is `false`.  Only available when using an output of type `http` and `custom`.
	// +kubebuilder:validation:Optional
	FluentBitKeepAnnotations *bool `json:"keepAnnotations,omitempty"`
	// Deprecated: The field is based on the Fluent Bit-based technology stack. Use the OpenTelemetry-based stack instead, see https://kyma-project.io/external-content/telemetry-manager/docs/user/integrate-otlp-backend/migration-to-otlp-logs.html.
	// FluentBitDropLabels defines whether to drop all Kubernetes labels. The default is `false`. Only available when using an output of type `http` and `custom`. For an `otlp` output, use the label enrichement feature in the Telemetry resource instead.
	// +kubebuilder:validation:Optional
	FluentBitDropLabels *bool `json:"dropLabels,omitempty"`
	// KeepOriginalBody retains the original log data if the log data is in JSON and it is successfully parsed. If set to `false`, the original log data is removed from the log record. The default is `true`.
	// +kubebuilder:validation:Optional
	KeepOriginalBody *bool `json:"keepOriginalBody,omitempty"`
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

// LogPipelineOutput configures the backend to which logs are sent. You must specify exactly one output per pipeline.
// +kubebuilder:validation:XValidation:rule="has(self.otlp) == has(oldSelf.otlp)", message="Switching to or away from OTLP output is not supported. Please re-create the LogPipeline instead"
// +kubebuilder:validation:XValidation:rule="(has(self.custom) == true ? 1 : 0) + (has(self.http) == true ? 1 : 0) + (has(self.otlp) == true ? 1 : 0) == 1",message="Exactly one output out of 'custom', 'http' or 'otlp' must be defined"
type LogPipelineOutput struct {
	// Deprecated: The field is based on the Fluent Bit-based technology stack. Use the OpenTelemetry-based stack instead, see https://kyma-project.io/external-content/telemetry-manager/docs/user/integrate-otlp-backend/migration-to-otlp-logs.html.
	// FluentBitCustom defines a custom output in the [Fluent Bit syntax](https://docs.fluentbit.io/manual/pipeline/outputs) where you want to push the logs. If you use a `custom` output, you put the LogPipeline in unsupported mode. Only available when using an output of type `http` and `custom`.
	// +kubebuilder:validation:Optional
	FluentBitCustom string `json:"custom,omitempty"`
	// Deprecated: The field is based on the Fluent Bit-based technology stack. Use the OpenTelemetry-based stack instead, see https://kyma-project.io/external-content/telemetry-manager/docs/user/integrate-otlp-backend/migration-to-otlp-logs.html.
	// FluentBitHTTP configures a FluentBitHTTP-based output compatible with the Fluent Bit FluentBitHTTP output plugin.
	// +kubebuilder:validation:Optional
	FluentBitHTTP *FluentBitHTTPOutput `json:"http,omitempty"`
	// OTLP defines an output using the OpenTelemetry protocol.
	// +kubebuilder:validation:Optional
	OTLP *OTLPOutput `json:"otlp,omitempty"`
}

// FluentBitHTTPOutput configures an HTTP-based output compatible with the Fluent Bit HTTP output plugin.
type FluentBitHTTPOutput struct {
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
	TLS OutputTLS `json:"tls"`
	// Dedot enables de-dotting of Kubernetes labels and annotations. For compatibility with OpenSearch-based backends, dots (.) are replaced by underscores (_). Default is `false`.
	// +kubebuilder:validation:Optional
	Dedot bool `json:"dedot,omitempty"`
}

// FluentBitFilter configures custom Fluent Bit `filters` to transform logs. Only available when using an output of type `http` and `custom`.
type FluentBitFilter struct {
	// Custom defines a custom filter in the [Fluent Bit syntax](https://docs.fluentbit.io/manual/pipeline/outputs). If you use a `custom` filter, you put the LogPipeline in unsupported mode. Only available when using an output of type `http` and `custom`.
	// +kubebuilder:validation:Optional
	Custom string `json:"custom,omitempty"`
}

// FluentBitFile provides file content to be consumed by a LogPipeline configuration
type FluentBitFile struct {
	// Name of the file under which the content is mounted in the Fluent Bit configuration.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Content of the file to be mounted in the Fluent Bit configuration.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Content string `json:"content"`
}

// FluentBitVariable references a Kubernetes secret that should be provided as environment variable to Fluent Bit
type FluentBitVariable struct {
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
