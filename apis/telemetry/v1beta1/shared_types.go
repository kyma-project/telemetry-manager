package v1beta1

import (
	"k8s.io/apimachinery/pkg/types"
)

type ValueType struct {
	// The value as plain text.
	Value string `json:"value,omitempty"`
	// The value as a reference to a resource.
	ValueFrom *ValueFromSource `json:"valueFrom,omitempty"`
}

func (v *ValueType) IsDefined() bool {
	if v == nil {
		return false
	}

	if v.Value != "" {
		return true
	}

	return v.ValueFrom != nil &&
		v.ValueFrom.SecretKeyRef != nil &&
		v.ValueFrom.SecretKeyRef.Name != "" &&
		v.ValueFrom.SecretKeyRef.Key != "" &&
		v.ValueFrom.SecretKeyRef.Namespace != ""
}

type ValueFromSource struct {
	// Refers to the value of a specific key in a Secret. You must provide `name` and `namespace` of the Secret, as well as the name of the `key`.
	SecretKeyRef *SecretKeyRef `json:"secretKeyRef,omitempty"`
}

type SecretKeyRef struct {
	// The name of the Secret containing the referenced value
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// The name of the Namespace containing the Secret with the referenced value.
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
	// The name of the attribute of the Secret holding the referenced value.
	// +kubebuilder:validation:Required
	Key string `json:"key"`
}

func (skr *SecretKeyRef) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: skr.Name, Namespace: skr.Namespace}
}

type Header struct {
	// Defines the header name.
	Name string `json:"name"`
	// Defines the header value.
	ValueType `json:",inline"`
	// Defines an optional header value prefix. The prefix is separated from the value by a space character.
	Prefix string `json:"prefix,omitempty"`
}

type OTLPProtocol string

const (
	OTLPProtocolHTTP OTLPProtocol = "http"
	OTLPProtocolGRPC OTLPProtocol = "grpc"
)

// OTLPOutput OTLP output configuration
// +kubebuilder:validation:XValidation:rule="((!has(self.path) || size(self.path) <= 0) && (has(self.protocol) && self.protocol == 'grpc')) || (has(self.protocol) && self.protocol == 'http')", message="Path is only available with HTTP protocol"
type OTLPOutput struct {
	// Defines the OTLP protocol (http or grpc). Default is grpc.
	// +kubebuilder:default:=grpc
	// +kubebuilder:validation:Enum=grpc;http
	Protocol OTLPProtocol `json:"protocol,omitempty"`
	// Defines the host and port (<host>:<port>) of an OTLP endpoint.
	// +kubebuilder:validation:Required
	Endpoint ValueType `json:"endpoint"`
	// Defines OTLP export URL path (only for the HTTP protocol). This value overrides auto-appended paths /v1/metrics and /v1/traces
	Path string `json:"path,omitempty"`
	// Defines authentication options for the OTLP output
	Authentication *AuthenticationOptions `json:"authentication,omitempty"`
	// Defines custom headers to be added to outgoing HTTP or GRPC requests.
	Headers []Header `json:"headers,omitempty"`
	// Defines TLS options for the OTLP output.
	TLS *OutputTLS `json:"tls,omitempty"`
}

type AuthenticationOptions struct {
	// Activates `Basic` authentication for the destination providing relevant Secrets.
	Basic *BasicAuthOptions `json:"basic,omitempty"`
}

type BasicAuthOptions struct {
	// Contains the basic auth username or a Secret reference.
	// +kubebuilder:validation:Required
	User ValueType `json:"user"`
	// Contains the basic auth password or a Secret reference.
	// +kubebuilder:validation:Required
	Password ValueType `json:"password"`
}
