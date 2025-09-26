package v1alpha1

// ValueType represents either a direct value or a reference to a value stored in a Secret.
// +kubebuilder:validation:XValidation:rule="has(self.value) != has(self.valueFrom)",message="Exactly one of 'value' or 'valueFrom' must be set"
type ValueType struct {
	// Value as plain text.
	// +kubebuilder:validation:Optional
	Value string `json:"value,omitempty"`
	// ValueFrom is the value as a reference to a resource.
	// +kubebuilder:validation:Optional
	ValueFrom *ValueFromSource `json:"valueFrom,omitempty"`
}

// ValueFromSource represents the different FromSource options for a ValueType.
type ValueFromSource struct {
	// SecretKeyRef refers to the value of a specific key in a Secret. You must provide `name` and `namespace` of the Secret, as well as the name of the `key`.
	// +kubebuilder:validation:Required
	SecretKeyRef *SecretKeyRef `json:"secretKeyRef"`
}

// SecretKeyRef selects a key of a Secret in the given namespace.
type SecretKeyRef struct {
	// Name of the Secret containing the referenced value.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Namespace containing the Secret with the referenced value.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`
	// Key defines the name of the attribute of the Secret holding the referenced value.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

const (
	OTLPProtocolHTTP string = "http"
	OTLPProtocolGRPC string = "grpc"
)

// OTLPOutput OTLP output configuration
// +kubebuilder:validation:XValidation:rule="((!has(self.path) || size(self.path) <= 0) && (has(self.protocol) && self.protocol == 'grpc')) || (has(self.protocol) && self.protocol == 'http')", message="Path is only available with HTTP protocol"
type OTLPOutput struct {
	// Protocol defines the OTLP protocol (`http` or `grpc`). Default is `grpc`.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=grpc;http
	Protocol string `json:"protocol,omitempty"`
	// Endpoint defines the host and port (`<host>:<port>`) of an OTLP endpoint.
	// +kubebuilder:validation:Required
	Endpoint ValueType `json:"endpoint"`
	// Path defines OTLP export URL path (only for the HTTP protocol). This value overrides auto-appended paths `/v1/metrics` and `/v1/traces`
	// +kubebuilder:validation:Optional
	Path string `json:"path,omitempty"`
	// Authentication defines authentication options for the OTLP output
	// +kubebuilder:validation:Optional
	Authentication *AuthenticationOptions `json:"authentication,omitempty"`
	// Headers defines custom headers to be added to outgoing HTTP or gRPC requests.
	// +kubebuilder:validation:Optional
	Headers []Header `json:"headers,omitempty"`
	// TLS defines TLS options for the OTLP output.
	// +kubebuilder:validation:Optional
	TLS *OTLPTLS `json:"tls,omitempty"`
}

type AuthenticationOptions struct {
	// Basic activates `Basic` authentication for the destination providing relevant Secrets.
	// +kubebuilder:validation:Optional
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
	// +kubebuilder:validation:Required
	ValueType `json:",inline"`

	// Name defines the header name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Prefix defines an optional header value prefix. The prefix is separated from the value by a space character.
	// +kubebuilder:validation:Optional
	Prefix string `json:"prefix,omitempty"`
}

// OTLPTLS defines the TLS configuration for an OTLP output.
// +kubebuilder:validation:XValidation:rule="has(self.cert) == has(self.key)", message="Can define either both 'cert' and 'key', or neither"
type OTLPTLS struct {
	// Insecure defines whether to send requests using plaintext instead of TLS.
	// +kubebuilder:validation:Optional
	Insecure bool `json:"insecure,omitempty"`
	// InsecureSkipVerify defines whether to skip server certificate verification when using TLS.
	// +kubebuilder:validation:Optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
	// Defines an optional CA certificate for server certificate verification when using TLS. The certificate must be provided in PEM format.
	// +kubebuilder:validation:Optional
	CA *ValueType `json:"ca,omitempty"`
	// Defines a client certificate to use when using TLS. The certificate must be provided in PEM format.
	// +kubebuilder:validation:Optional
	Cert *ValueType `json:"cert,omitempty"`
	// Defines the client key to use when using TLS. The key must be provided in PEM format.
	// +kubebuilder:validation:Optional
	Key *ValueType `json:"key,omitempty"`
}

// OTLPInput defines the collection of push-based metrics that use the OpenTelemetry protocol.
type OTLPInput struct {
	// If set to `true`, no push-based OTLP signals are collected. The default is `false`.
	// +kubebuilder:validation:Optional
	Disabled bool `json:"disabled,omitempty"`
	// Namespaces describes whether push-based OTLP signals from specific namespaces are selected. System namespaces are enabled by default.
	// +kubebuilder:validation:Optional
	Namespaces *NamespaceSelector `json:"namespaces,omitempty"`
}

// NamespaceSelector describes whether signals from specific namespaces are selected.
// +kubebuilder:validation:MaxProperties=1
type NamespaceSelector struct {
	// Include signals from the specified Namespace names only.
	// +kubebuilder:validation:Optional
	Include []string `json:"include,omitempty"`
	// Exclude signals from the specified Namespace names only.
	// +kubebuilder:validation:Optional
	Exclude []string `json:"exclude,omitempty"`
}

// TransformSpec defines a transformation to apply to telemetry data.
type TransformSpec struct {
	// Conditions specify a list of multiple where clauses, which will be processed as global conditions for the accompanying set of statements. The conditions are ORed together, which means only one condition needs to evaluate to true in order for the statements (including their individual where clauses) to be executed.
	// +kubebuilder:validation:Optional
	Conditions []string `json:"conditions,omitempty"`
	// Statements specify a list of OTTL statements to perform the transformation.
	// +kubebuilder:validation:Optional
	Statements []string `json:"statements,omitempty"`
}
