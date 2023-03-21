package v1alpha1

import (
	"k8s.io/apimachinery/pkg/types"
)

type ValueType struct {
	Value     string           `json:"value,omitempty"`
	ValueFrom *ValueFromSource `json:"valueFrom,omitempty"`
}

func (v *ValueType) IsDefined() bool {
	if v.Value != "" {
		return true
	}

	return v.ValueFrom != nil && v.ValueFrom.IsSecretKeyRef()
}

type ValueFromSource struct {
	// 'Reference to a key in a Secret. You must provide `name` and `namespace` of the Secret, as well as the name of the `key`.'
	SecretKeyRef *SecretKeyRef `json:"secretKeyRef,omitempty"`
}

func (v *ValueFromSource) IsSecretKeyRef() bool {
	return v.SecretKeyRef != nil && v.SecretKeyRef.Name != "" && v.SecretKeyRef.Key != ""
}

type SecretKeyRef struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Key       string `json:"key,omitempty"`
}

func (skr *SecretKeyRef) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: skr.Name, Namespace: skr.Namespace}
}

type LogPipelineValidationConfig struct {
	DeniedOutPutPlugins []string
	DeniedFilterPlugins []string
}
type Header struct {
	// Defines the header name
	Name string `json:"name"`
	// Defines the header value
	ValueType `json:",inline"`
}

type OtlpOutput struct {
	// Defines the OTLP protocol (http or grpc).
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:default:=grpc
	// +kubebuilder:validation:Enum=grpc;http
	Protocol string `json:"protocol,omitempty"`
	// Defines the host and port (<host>:<port>) of an OTLP endpoint.
	// +kubebuilder:validation:Required
	Endpoint ValueType `json:"endpoint"`
	// Defines authentication options for the OTLP output
	Authentication *AuthenticationOptions `json:"authentication,omitempty"`
	// Custom headers to be added to outgoing HTTP or GRPC requests
	Headers []Header `json:"headers,omitempty"`
}

type AuthenticationOptions struct {
	// Contains credentials for HTTP basic auth
	Basic *BasicAuthOptions `json:"basic,omitempty"`
}

type BasicAuthOptions struct {
	// Contains the basic auth username or a secret reference
	// +kubebuilder:validation:Required
	User ValueType `json:"user"`
	// Contains the basic auth password or a secret reference
	// +kubebuilder:validation:Required
	Password ValueType `json:"password"`
}

func (b *BasicAuthOptions) IsDefined() bool {
	return b.User.IsDefined() && b.Password.IsDefined()
}
