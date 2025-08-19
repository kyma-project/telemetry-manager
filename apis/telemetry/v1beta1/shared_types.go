package v1beta1

type ValueType struct {
	// Value as plain text.
	Value string `json:"value,omitempty"`
	// ValueFrom is the value as a reference to a resource.
	ValueFrom *ValueFromSource `json:"valueFrom,omitempty"`
}

type ValueFromSource struct {
	// SecretKeyRef refers to the value of a specific key in a Secret. You must provide `name` and `namespace` of the Secret, as well as the name of the `key`.
	SecretKeyRef *SecretKeyRef `json:"secretKeyRef,omitempty"`
}

type SecretKeyRef struct {
	// Name of the Secret containing the referenced value.
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`
	// Namespace containing the Secret with the referenced value.
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace,omitempty"`
	// Key defines the name of the attribute of the Secret holding the referenced value.
	// +kubebuilder:validation:Required
	Key string `json:"key,omitempty"`
}

type OTLPProtocol string

const (
	OTLPProtocolHTTP OTLPProtocol = "http"
	OTLPProtocolGRPC OTLPProtocol = "grpc"
)

// OTLPOutput OTLP output configuration
// +kubebuilder:validation:XValidation:rule="((!has(self.path) || size(self.path) <= 0) && (has(self.protocol) && self.protocol == 'grpc')) || (has(self.protocol) && self.protocol == 'http')", message="Path is only available with HTTP protocol"
type OTLPOutput struct {
	// Protocol defines the OTLP protocol (http or grpc). Default is grpc.
	// +kubebuilder:validation:Enum=grpc;http
	Protocol OTLPProtocol `json:"protocol,omitempty"`
	// Endpoint defines the host and port (<host>:<port>) of an OTLP endpoint.
	// +kubebuilder:validation:Required
	Endpoint ValueType `json:"endpoint"`
	// Path defines OTLP export URL path (only for the HTTP protocol). This value overrides auto-appended paths `/v1/metrics` and `/v1/traces`
	Path string `json:"path,omitempty"`
	// Authentication defines authentication options for the OTLP output
	Authentication *AuthenticationOptions `json:"authentication,omitempty"`
	// Headers defines custom headers to be added to outgoing HTTP or GRPC requests.
	Headers []Header `json:"headers,omitempty"`
	// TLS defines TLS options for the OTLP output.
	TLS *OutputTLS `json:"tls,omitempty"`
}

type AuthenticationOptions struct {
	// Basic activates `Basic` authentication for the destination providing relevant Secrets.
	Basic *BasicAuthOptions `json:"basic,omitempty"`
}

type BasicAuthOptions struct {
	// User contains the basic auth username or a Secret reference.
	// +kubebuilder:validation:Required
	User ValueType `json:"user"`
	// Password contains the basic auth password or a Secret reference.
	// +kubebuilder:validation:Required
	Password ValueType `json:"password"`
}

type Header struct {
	// Defines the header value.
	ValueType `json:",inline"`

	// Name defines the header name.
	Name string `json:"name"`
	// Prefix defines an optional header value prefix. The prefix is separated from the value by a space character.
	Prefix string `json:"prefix,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="has(self.cert) == has(self.key)", message="Can define either both 'cert' and 'key', or neither"
type OutputTLS struct {
	// Disabled specifies if TLS is disabled or enabled. Default is `false`.
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

// OTLPInput defines the collection of push-based metrics that use the OpenTelemetry protocol.
type OTLPInput struct {
	// If set to `true`, no push-based OTLP signals are collected. The default is `false`.
	Disabled bool `json:"disabled,omitempty"`
	// Namespaces describes whether push-based OTLP signals from specific namespaces are selected. System namespaces are enabled by default.
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

// TransformSpec defines a transformation to apply to telemetry data.
type TransformSpec struct {
	// Conditions specify a list of multiple where clauses, which will be processed as global conditions for the accompanying set of statements. The conditions are ORed together, which means only one condition needs to evaluate to true in order for the statements (including their individual where clauses) to be executed.
	// +optional
	Conditions []string `json:"conditions,omitempty"`
	// Statements specify a list of OTTL statements to perform the transformation.
	// +optional
	Statements []string `json:"statements,omitempty"`
}
