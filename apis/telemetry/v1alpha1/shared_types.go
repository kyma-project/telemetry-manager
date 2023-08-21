package v1alpha1

import (
	"k8s.io/apimachinery/pkg/types"
)

type ValueType struct {
	// Value that can contain references to Secret values.
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
	// Refers to a key in a Secret. You must provide `name` and `namespace` of the Secret, as well as the name of the `key`.
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
	// Defines the header name.
	Name string `json:"name"`
	// Defines the header value.
	ValueType `json:",inline"`
}

type OtlpTLS struct {
	// Defines whether to send requests via plaintext instead of TLS. This setting is ignored when the OTLP endpoint specifies a valid scheme.
	// +kubebuilder:default:=false
	Insecure bool `json:"insecure"`
	// Defines whether to skip TLS certificate verification.
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
	// Defines the CA certificate.
	CA ValueType `json:"ca,omitempty"`
	// Defines the client certificate.
	Cert ValueType `json:"cert,omitempty"`
	// Defines the client key.
	Key ValueType `json:"key,omitempty"`
}

type OtlpOutput struct {
	// Defines the OTLP protocol (http or grpc). Default is GRPC.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:default:=grpc
	// +kubebuilder:validation:Enum=grpc;http
	Protocol string `json:"protocol,omitempty"`
	// Defines the host and port (<host>:<port>) of an OTLP endpoint. If the endpoint scheme is `https` or `http`, this setting takes precedence over the `tls.insecure` configuration setting.
	// +kubebuilder:validation:Required
	Endpoint ValueType `json:"endpoint"`
	// Defines authentication options for the OTLP output
	Authentication *AuthenticationOptions `json:"authentication,omitempty"`
	// Defines custom headers to be added to outgoing HTTP or GRPC requests.
	Headers []Header `json:"headers,omitempty"`
	// Defines TLS options for the OTLP output.
	TLS OtlpTLS `json:"tls"`
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
