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

// LogPipelineSpec defines the desired state of LogPipeline
type LogPipelineSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Defines where to collect logs, including selector mechanisms.
	Input   Input    `json:"input,omitempty"`
	Filters []Filter `json:"filters,omitempty"`
	// [Fluent Bit output](https://docs.fluentbit.io/manual/pipeline/outputs) where you want to push the logs. Only one output can be specified.
	Output Output      `json:"output,omitempty"`
	Files  []FileMount `json:"files,omitempty"`
	// A list of mappings from Kubernetes Secret keys to environment variables. Mapped keys are mounted as environment variables, so that they are available as [Variables](https://docs.fluentbit.io/manual/administration/configuring-fluent-bit/classic-mode/variables) in the sections.
	Variables []VariableRef `json:"variables,omitempty"`
}

// Input describes a log input for a LogPipeline.
type Input struct {
	// Configures in more detail from which containers application logs are enabled as input.
	Application ApplicationInput `json:"application,omitempty"`
}

// ApplicationInput specifies the default type of Input that handles application logs from runtime containers. It configures in more detail from which containers logs are selected as input.
type ApplicationInput struct {
	// Describes whether application logs from specific Namespaces are selected. The options are mutually exclusive. System Namespaces are excluded by default from the collection.
	Namespaces InputNamespaces `json:"namespaces,omitempty"`
	// Describes whether application logs from specific containers are selected. The options are mutually exclusive.
	Containers InputContainers `json:"containers,omitempty"`
	// Defines whether to keep all Kubernetes annotations. The default is `false`.
	KeepAnnotations bool `json:"keepAnnotations,omitempty"`
	// Defines whether to drop all Kubernetes labels. The default is `false`.
	DropLabels bool `json:"dropLabels,omitempty"`
}

// InputNamespaces describes whether application logs from specific Namespaces are selected. The options are mutually exclusive. System Namespaces are excluded by default from the collection.
type InputNamespaces struct {
	// Include only the container logs of the specified Namespace names.
	Include []string `json:"include,omitempty"`
	// Exclude the container logs of the specified Namespace names.
	Exclude []string `json:"exclude,omitempty"`
	// Set to `true` if collecting from all Namespaces must also include the system Namespaces like kube-system, istio-system, and kyma-system.
	System bool `json:"system,omitempty"`
}

// InputContainers describes whether application logs from specific containers are selected. The options are mutually exclusive.
type InputContainers struct {
	// Specifies to include only the container logs with the specified container names.
	Include []string `json:"include,omitempty"`
	// Specifies to exclude only the container logs with the specified container names.
	Exclude []string `json:"exclude,omitempty"`
}

// Describes a filtering option on the logs of the pipeline.
type Filter struct {
	// Custom filter definition in the Fluent Bit syntax. Note: If you use a `custom` filter, you put the LogPipeline in unsupported mode.
	Custom string `json:"custom,omitempty"`
}

// LokiOutput configures an output to the Kyma-internal Loki instance.
type LokiOutput struct {
	// Grafana Loki URL.
	URL ValueType `json:"url,omitempty"`
	// Labels to set for each log record.
	Labels map[string]string `json:"labels,omitempty"`
	// Attributes to be removed from a log record.
	RemoveKeys []string `json:"removeKeys,omitempty"`
}

// HTTPOutput configures an HTTP-based output compatible with the Fluent Bit HTTP output plugin.
type HTTPOutput struct {
	// Defines the host of the HTTP receiver.
	Host ValueType `json:"host,omitempty"`
	// Defines the basic auth user.
	User ValueType `json:"user,omitempty"`
	// Defines the basic auth password.
	Password ValueType `json:"password,omitempty"`
	// Defines the URI of the HTTP receiver. Default is "/".
	URI string `json:"uri,omitempty"`
	// Defines the port of the HTTP receiver. Default is 443.
	Port string `json:"port,omitempty"`
	// Defines the compression algorithm to use.
	Compress string `json:"compress,omitempty"`
	// Data format to be used in the HTTP request body. Default is `json`.
	Format string `json:"format,omitempty"`
	// Configures TLS for the HTTP target server.
	TLSConfig TLSConfig `json:"tls,omitempty"`
	// Enables de-dotting of Kubernetes labels and annotations for compatibility with ElasticSearch based backends. Dots (.) will be replaced by underscores (_). Default is `false`.
	Dedot bool `json:"dedot,omitempty"`
}

type TLSConfig struct {
	// Indicates if TLS is disabled or enabled. Default is `false`.
	Disabled bool `json:"disabled,omitempty"`
	// If `true`, the validation of certificates is skipped. Default is `false`.
	SkipCertificateValidation bool `json:"skipCertificateValidation,omitempty"`
	// Defines an optional CA certificate for server certificate verification when using TLS. The certificate must be provided in PEM format.
	CA *ValueType `json:"ca,omitempty"`
	// Defines a client certificate to use when using TLS. The certificate must be provided in PEM format.
	Cert *ValueType `json:"cert,omitempty"`
	// Defines the client key to use when using TLS. The key must be provided in PEM format.
	Key *ValueType `json:"key,omitempty"`
}

// Output describes a Fluent Bit output configuration section.
type Output struct {
	// Defines a custom output in the Fluent Bit syntax. Note: If you use a `custom` output, you put the LogPipeline in unsupported mode.
	Custom string `json:"custom,omitempty"`
	// Configures an HTTP-based output compatible with the Fluent Bit HTTP output plugin.
	HTTP *HTTPOutput `json:"http,omitempty"`
	// The grafana-loki output is not supported anymore. For integration with a custom Loki installation, use the `custom` output and follow [Installing a custom Loki stack in Kyma](https://kyma-project.io/#/telemetry-manager/user/integration/loki/README ).
	Loki *LokiOutput `json:"grafana-loki,omitempty"`
}

func (i *Input) IsDefined() bool {
	return i != nil
}

func (o *Output) IsCustomDefined() bool {
	return o.Custom != ""
}

func (o *Output) IsHTTPDefined() bool {
	return o.HTTP != nil && o.HTTP.Host.IsDefined()
}

func (o *Output) IsLokiDefined() bool {
	return o.Loki != nil && o.Loki.URL.IsDefined()
}

func (o *Output) IsAnyDefined() bool {
	return o.pluginCount() > 0
}

func (o *Output) IsSingleDefined() bool {
	return o.pluginCount() == 1
}

func (o *Output) pluginCount() int {
	plugins := 0
	if o.IsCustomDefined() {
		plugins++
	}
	if o.IsHTTPDefined() {
		plugins++
	}
	if o.IsLokiDefined() {
		plugins++
	}
	return plugins
}

// Provides file content to be consumed by a LogPipeline configuration
type FileMount struct {
	Name    string `json:"name,omitempty"`
	Content string `json:"content,omitempty"`
}

// References a Kubernetes secret that should be provided as environment variable to Fluent Bit
type VariableRef struct {
	// Name of the variable to map.
	Name      string          `json:"name,omitempty"`
	ValueFrom ValueFromSource `json:"valueFrom,omitempty"`
}

// LogPipelineStatus shows the observed state of the LogPipeline
type LogPipelineStatus struct {
	// An array of conditions describing the status of the pipeline.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Is active when the LogPipeline uses a `custom` output or filter; see [unsupported mode](https://github.com/kyma-project/telemetry-manager/blob/main/docs/user/02-logs.md#unsupported-mode).
	UnsupportedMode bool `json:"unsupportedMode,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Configuration Generated",type=string,JSONPath=`.status.conditions[?(@.type=="ConfigurationGenerated")].status`
// +kubebuilder:printcolumn:name="Agent Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="AgentHealthy")].status`
// +kubebuilder:printcolumn:name="Unsupported-Mode",type=boolean,JSONPath=`.status.unsupportedMode`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// LogPipeline is the Schema for the logpipelines API
type LogPipeline struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Defines the desired state of LogPipeline
	Spec LogPipelineSpec `json:"spec,omitempty"`
	// Shows the observed state of the LogPipeline
	Status LogPipelineStatus `json:"status,omitempty"`
}

// ContainsCustomPlugin returns true if the pipeline contains any custom filters or outputs
func (lp *LogPipeline) ContainsCustomPlugin() bool {
	for _, filter := range lp.Spec.Filters {
		if filter.Custom != "" {
			return true
		}
	}
	return lp.Spec.Output.IsCustomDefined()
}

// +kubebuilder:object:root=true
// LogPipelineList contains a list of LogPipeline
type LogPipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LogPipeline `json:"items"`
}

//nolint:gochecknoinits // SchemeBuilder's registration is required.
func init() {
	SchemeBuilder.Register(&LogPipeline{}, &LogPipelineList{})
}
