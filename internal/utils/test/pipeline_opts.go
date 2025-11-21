package test

import (
	"strconv"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type OTLPOutputOption func(*telemetryv1alpha1.OTLPOutput)

func OTLPEndpoint(endpoint string) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OTLPOutput) {
		output.Endpoint = telemetryv1alpha1.ValueType{Value: endpoint}
	}
}

func OTLPEndpointFromSecret(secretName, secretNamespace, endpointKey string) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OTLPOutput) {
		output.Endpoint = telemetryv1alpha1.ValueType{
			ValueFrom: &telemetryv1alpha1.ValueFromSource{
				SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
					Name:      secretName,
					Namespace: secretNamespace,
					Key:       endpointKey,
				},
			},
		}
	}
}

func OTLPBasicAuth(user, password string) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OTLPOutput) {
		output.Authentication = &telemetryv1alpha1.AuthenticationOptions{
			Basic: &telemetryv1alpha1.BasicAuthOptions{
				User:     telemetryv1alpha1.ValueType{Value: user},
				Password: telemetryv1alpha1.ValueType{Value: password},
			},
		}
	}
}

func OTLPBasicAuthFromSecret(secretName, secretNamespace, userKey, passwordKey string) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OTLPOutput) {
		output.Authentication = &telemetryv1alpha1.AuthenticationOptions{
			Basic: &telemetryv1alpha1.BasicAuthOptions{
				User: telemetryv1alpha1.ValueType{
					ValueFrom: &telemetryv1alpha1.ValueFromSource{
						SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
							Name:      secretName,
							Namespace: secretNamespace,
							Key:       userKey,
						},
					},
				},
				Password: telemetryv1alpha1.ValueType{
					ValueFrom: &telemetryv1alpha1.ValueFromSource{
						SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
							Name:      secretName,
							Namespace: secretNamespace,
							Key:       passwordKey,
						},
					},
				},
			},
		}
	}
}

func OTLPCustomHeader(name, value, prefix string) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OTLPOutput) {
		output.Headers = append(output.Headers, telemetryv1alpha1.Header{
			Name: name,
			ValueType: telemetryv1alpha1.ValueType{
				Value: value,
			},
			Prefix: prefix,
		})
	}
}

func OTLPClientTLSFromString(ca, cert, key string) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OTLPOutput) {
		output.TLS = &telemetryv1alpha1.OTLPTLS{
			CA:   &telemetryv1alpha1.ValueType{Value: ca},
			Cert: &telemetryv1alpha1.ValueType{Value: cert},
			Key:  &telemetryv1alpha1.ValueType{Value: key},
		}
	}
}

func OTLPClientTLS(tls *telemetryv1alpha1.OTLPTLS) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OTLPOutput) {
		output.TLS = tls
	}
}

func OTLPProtocol(protocol string) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OTLPOutput) {
		output.Protocol = protocol
	}
}

func OTLPEndpointPath(path string) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OTLPOutput) {
		output.Path = path
	}
}

type OAuth2Option func(oauth2 *telemetryv1alpha1.OAuth2Options)

func OAuth2ClientID(clientID string) OAuth2Option {
	return func(oauth2 *telemetryv1alpha1.OAuth2Options) {
		oauth2.ClientID = telemetryv1alpha1.ValueType{Value: clientID}
	}
}

func OAuth2ClientIDFromSecret(secretName, secretNamespace, key string) OAuth2Option {
	return func(oauth2 *telemetryv1alpha1.OAuth2Options) {
		oauth2.ClientID = telemetryv1alpha1.ValueType{
			ValueFrom: &telemetryv1alpha1.ValueFromSource{
				SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
					Name:      secretName,
					Namespace: secretNamespace,
					Key:       key,
				},
			},
		}
	}
}

func OAuth2ClientSecret(clientSecret string) OAuth2Option {
	return func(oauth2 *telemetryv1alpha1.OAuth2Options) {
		oauth2.ClientSecret = telemetryv1alpha1.ValueType{Value: clientSecret}
	}
}

func OAuth2ClientSecretFromSecret(secretName, secretNamespace, key string) OAuth2Option {
	return func(oauth2 *telemetryv1alpha1.OAuth2Options) {
		oauth2.ClientSecret = telemetryv1alpha1.ValueType{
			ValueFrom: &telemetryv1alpha1.ValueFromSource{
				SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
					Name:      secretName,
					Namespace: secretNamespace,
					Key:       key,
				},
			},
		}
	}
}

func OAuth2TokenURL(tokenURL string) OAuth2Option {
	return func(oauth2 *telemetryv1alpha1.OAuth2Options) {
		oauth2.TokenURL = telemetryv1alpha1.ValueType{Value: tokenURL}
	}
}

func OAuth2TokenURLFromSecret(secretName, secretNamespace, key string) OAuth2Option {
	return func(oauth2 *telemetryv1alpha1.OAuth2Options) {
		oauth2.TokenURL = telemetryv1alpha1.ValueType{
			ValueFrom: &telemetryv1alpha1.ValueFromSource{
				SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
					Name:      secretName,
					Namespace: secretNamespace,
					Key:       key,
				},
			},
		}
	}
}

func OAuth2Scopes(scopes []string) OAuth2Option {
	return func(oauth2 *telemetryv1alpha1.OAuth2Options) {
		oauth2.Scopes = scopes
	}
}

func OAuth2Params(params map[string]string) OAuth2Option {
	return func(oauth2 *telemetryv1alpha1.OAuth2Options) {
		oauth2.Params = params
	}
}

type HTTPOutputOption func(output *telemetryv1alpha1.LogPipelineHTTPOutput)

func HTTPClientTLSFromString(ca, cert, key string) HTTPOutputOption {
	return func(output *telemetryv1alpha1.LogPipelineHTTPOutput) {
		output.TLS = telemetryv1alpha1.LogPipelineOutputTLS{
			CA:   &telemetryv1alpha1.ValueType{Value: ca},
			Cert: &telemetryv1alpha1.ValueType{Value: cert},
			Key:  &telemetryv1alpha1.ValueType{Value: key},
		}
	}
}

func HTTPClientTLS(tls telemetryv1alpha1.LogPipelineOutputTLS) HTTPOutputOption {
	return func(output *telemetryv1alpha1.LogPipelineHTTPOutput) {
		output.TLS = tls
	}
}

func HTTPHost(host string) HTTPOutputOption {
	return func(output *telemetryv1alpha1.LogPipelineHTTPOutput) {
		output.Host = telemetryv1alpha1.ValueType{Value: host}
	}
}

func HTTPHostFromSecret(secretName, secretNamespace, key string) HTTPOutputOption {
	return func(output *telemetryv1alpha1.LogPipelineHTTPOutput) {
		output.Host = telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      secretName,
			Namespace: secretNamespace,
			Key:       key,
		}}}
	}
}

func HTTPPort(port int32) HTTPOutputOption {
	return func(output *telemetryv1alpha1.LogPipelineHTTPOutput) {
		output.Port = strconv.Itoa(int(port))
	}
}

func HTTPDedot(dedot bool) HTTPOutputOption {
	return func(output *telemetryv1alpha1.LogPipelineHTTPOutput) {
		output.Dedot = dedot
	}
}

func HTTPBasicAuthFromSecret(secretName, secretNamespace, userKey, passwordKey string) HTTPOutputOption {
	return func(output *telemetryv1alpha1.LogPipelineHTTPOutput) {
		output.User = &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      secretName,
			Namespace: secretNamespace,
			Key:       userKey,
		}}}

		output.Password = &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      secretName,
			Namespace: secretNamespace,
			Key:       passwordKey,
		}}}
	}
}

type NamespaceSelectorOptions func(selector *telemetryv1alpha1.NamespaceSelector)

func IncludeNamespaces(namespaces ...string) NamespaceSelectorOptions {
	return func(selector *telemetryv1alpha1.NamespaceSelector) {
		selector.Include = namespaces
	}
}

func ExcludeNamespaces(namespaces ...string) NamespaceSelectorOptions {
	return func(selector *telemetryv1alpha1.NamespaceSelector) {
		selector.Exclude = namespaces
	}
}

// ExtendedNamespaceSelectorOptions unlike NamespaceSelectorOptions, allows to set the System flag
// Only used in the LogPipeline Application input, and will be deprecated in the future
type ExtendedNamespaceSelectorOptions func(selector *telemetryv1alpha1.LogPipelineNamespaceSelector)

func ExtIncludeNamespaces(namespaces ...string) ExtendedNamespaceSelectorOptions {
	return func(selector *telemetryv1alpha1.LogPipelineNamespaceSelector) {
		selector.Include = namespaces
	}
}

func ExtExcludeNamespaces(namespaces ...string) ExtendedNamespaceSelectorOptions {
	return func(selector *telemetryv1alpha1.LogPipelineNamespaceSelector) {
		selector.Exclude = namespaces
	}
}

func ExtSystemNamespaces(enable bool) ExtendedNamespaceSelectorOptions {
	return func(selector *telemetryv1alpha1.LogPipelineNamespaceSelector) {
		selector.System = enable
	}
}
