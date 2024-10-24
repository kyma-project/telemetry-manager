package v1alpha1

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

	return v.ValueFrom != nil && v.ValueFrom.IsSecretKeyRef()
}

type ValueFromSource struct {
	// Refers to the value of a specific key in a Secret. You must provide `name` and `namespace` of the Secret, as well as the name of the `key`.
	SecretKeyRef *SecretKeyRef `json:"secretKeyRef,omitempty"`
}

func (v *ValueFromSource) IsSecretKeyRef() bool {
	return v.SecretKeyRef != nil && v.SecretKeyRef.Name != "" && v.SecretKeyRef.Key != ""
}

type SecretKeyRef struct {
	// The name of the Secret containing the referenced value
	Name string `json:"name,omitempty"`
	// The name of the Namespace containing the Secret with the referenced value.
	Namespace string `json:"namespace,omitempty"`
	// The name of the attribute of the Secret holding the referenced value.
	Key string `json:"key,omitempty"`
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

// +kubebuilder:validation:XValidation:rule="has(self.cert) == has(self.key)", message="Can define either both 'cert' and 'key', or neither"
type OtlpTLS struct {
	// Defines whether to send requests using plaintext instead of TLS.
	Insecure bool `json:"insecure,omitempty"`
	// Defines whether to skip server certificate verification when using TLS.
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
	// Defines an optional CA certificate for server certificate verification when using TLS. The certificate must be provided in PEM format.
	CA *ValueType `json:"ca,omitempty"`
	// Defines a client certificate to use when using TLS. The certificate must be provided in PEM format.
	Cert *ValueType `json:"cert,omitempty"`
	// Defines the client key to use when using TLS. The key must be provided in PEM format.
	Key *ValueType `json:"key,omitempty"`
}

const (
	OtlpProtocolHTTP string = "http"
	OtlpProtocolGRPC string = "grpc"
)

// OtlpOutput OTLP output configuration
// +kubebuilder:validation:XValidation:rule="((!has(self.path) || size(self.path) <= 0) && (has(self.protocol) && self.protocol == 'grpc')) || (has(self.protocol) && self.protocol == 'http')", message="Path is only available with HTTP protocol"
type OtlpOutput struct {
	// Defines the OTLP protocol (http or grpc). Default is grpc.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:default:=grpc
	// +kubebuilder:validation:Enum=grpc;http
	Protocol string `json:"protocol,omitempty"`
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
	TLS *OtlpTLS `json:"tls,omitempty"`
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

func (b *BasicAuthOptions) IsDefined() bool {
	return b.User.IsDefined() && b.Password.IsDefined()
}

// OTLPInput defines the collection of push-based metrics that use the OpenTelemetry protocol.
type OTLPInput struct {
	// If disabled, push-based OTLP signals are not collected. The default is `false`.
	Disabled bool `json:"disabled,omitempty"`
	// Describes whether push-based OTLP signals from specific namespaces are selected. System namespaces are enabled by default.
	// +optional
	Namespaces *NamespaceSelector `json:"namespaces,omitempty"`
}

// NamespaceSelector describes whether signals from specific namespaces are selected.
// +kubebuilder:validation:XValidation:rule="!((has(self.include) && size(self.include) != 0) && (has(self.exclude) && size(self.exclude) != 0))", message="Can only define one namespace selector - either 'include' or 'exclude'"
type NamespaceSelector struct {
	// Include signals from the specified Namespace names only.
	Include []string `json:"include,omitempty"`
	// Exclude signals from the specified Namespace names only.
	Exclude []string `json:"exclude,omitempty"`
}
