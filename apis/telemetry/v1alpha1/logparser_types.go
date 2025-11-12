package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//nolint:gochecknoinits // SchemeBuilder's registration is required.
func init() {
	SchemeBuilder.Register(&LogParser{}, &LogParserList{})
}

// LogParserList contains a list of LogParser.
// +kubebuilder:object:root=true
type LogParserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []LogParser `json:"items"`
}

// LogParser is the Schema for the logparsers API.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:metadata:labels={app.kubernetes.io/component=controller,app.kubernetes.io/managed-by=Helm,app.kubernetes.io/name=telemetry-manager,app.kubernetes.io/part-of=telemetry,kyma-project.io/module=telemetry}
// +kubebuilder:metadata:annotations={meta.helm.sh/release-name=telemetry,meta.helm.sh/release-namespace=kyma-system}
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Agent Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="AgentHealthy")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:deprecatedversion:warning="The LogParser API is deprecated. Instead, log in JSON format and use the JSON parsing feature of the LogPipeline"
type LogParser struct {
	metav1.TypeMeta `json:",inline"`
	// +kubebuilder:validation:Optional
	metav1.ObjectMeta `json:"metadata"`

	// Defines the desired state of LogParser.
	// +kubebuilder:validation:Optional
	Spec LogParserSpec `json:"spec"`
	// Shows the observed state of the LogParser.
	// +kubebuilder:validation:Optional
	Status LogParserStatus `json:"status"`
}

// LogParserSpec defines the desired state of LogParser.
type LogParserSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// [Fluent Bit Parsers](https://docs.fluentbit.io/manual/pipeline/parsers). The parser specified here has no effect until it is referenced by a [Pod annotation](https://docs.fluentbit.io/manual/pipeline/filters/kubernetes#kubernetes-annotations) on your workload or by a [Parser Filter](https://docs.fluentbit.io/manual/pipeline/filters/parser) defined in a pipeline's filters section.
	Parser string `json:"parser,omitempty"`
}

// LogParserStatus shows the observed state of the LogParser.
type LogParserStatus struct {
	// An array of conditions describing the status of the parser.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
