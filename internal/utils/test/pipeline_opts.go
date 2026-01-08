package test

import (
	"strconv"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

type OTLPOutputOption func(*telemetryv1beta1.OTLPOutput)

func OTLPEndpoint(endpoint string) OTLPOutputOption {
	return func(output *telemetryv1beta1.OTLPOutput) {
		output.Endpoint = telemetryv1beta1.ValueType{Value: endpoint}
	}
}

func OTLPOAuth2(oauth2Opts ...OAuth2Option) OTLPOutputOption {
	return func(output *telemetryv1beta1.OTLPOutput) {
		oauth2opts := &telemetryv1beta1.OAuth2Options{}
		for _, opt := range oauth2Opts {
			opt(oauth2opts)
		}

		output.Authentication = &telemetryv1beta1.AuthenticationOptions{
			OAuth2: oauth2opts,
		}
	}
}

func OTLPEndpointFromSecret(secretName, secretNamespace, endpointKey string) OTLPOutputOption {
	return func(output *telemetryv1beta1.OTLPOutput) {
		output.Endpoint = telemetryv1beta1.ValueType{
			ValueFrom: &telemetryv1beta1.ValueFromSource{
				SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
					Name:      secretName,
					Namespace: secretNamespace,
					Key:       endpointKey,
				},
			},
		}
	}
}

func OTLPBasicAuth(user, password string) OTLPOutputOption {
	return func(output *telemetryv1beta1.OTLPOutput) {
		output.Authentication = &telemetryv1beta1.AuthenticationOptions{
			Basic: &telemetryv1beta1.BasicAuthOptions{
				User:     telemetryv1beta1.ValueType{Value: user},
				Password: telemetryv1beta1.ValueType{Value: password},
			},
		}
	}
}

func OTLPBasicAuthFromSecret(secretName, secretNamespace, userKey, passwordKey string) OTLPOutputOption {
	return func(output *telemetryv1beta1.OTLPOutput) {
		output.Authentication = &telemetryv1beta1.AuthenticationOptions{
			Basic: &telemetryv1beta1.BasicAuthOptions{
				User: telemetryv1beta1.ValueType{
					ValueFrom: &telemetryv1beta1.ValueFromSource{
						SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
							Name:      secretName,
							Namespace: secretNamespace,
							Key:       userKey,
						},
					},
				},
				Password: telemetryv1beta1.ValueType{
					ValueFrom: &telemetryv1beta1.ValueFromSource{
						SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
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
	return func(output *telemetryv1beta1.OTLPOutput) {
		output.Headers = append(output.Headers, telemetryv1beta1.Header{
			Name: name,
			ValueType: telemetryv1beta1.ValueType{
				Value: value,
			},
			Prefix: prefix,
		})
	}
}

// OTLPClientMTLSFromString sets the mTLS configuration for the OTLP output
func OTLPClientMTLSFromString(ca, cert, key string) OTLPOutputOption {
	return func(output *telemetryv1beta1.OTLPOutput) {
		output.TLS = &telemetryv1beta1.OutputTLS{
			CA:   &telemetryv1beta1.ValueType{Value: ca},
			Cert: &telemetryv1beta1.ValueType{Value: cert},
			Key:  &telemetryv1beta1.ValueType{Value: key},
		}
	}
}

// OTLPClientTLS sets the TLS configuration for the OTLP output (it does not include client certs)
func OTLPClientTLS(tls *telemetryv1beta1.OutputTLS) OTLPOutputOption {
	return func(output *telemetryv1beta1.OTLPOutput) {
		output.TLS = tls
	}
}

func OTLPClientTLSFromString(ca string) OTLPOutputOption {
	return func(output *telemetryv1beta1.OTLPOutput) {
		output.TLS = &telemetryv1beta1.OutputTLS{
			CA: &telemetryv1beta1.ValueType{Value: ca},
		}
	}
}

func OTLPInsecure(insecure bool) OTLPOutputOption {
	return func(output *telemetryv1beta1.OTLPOutput) {
		if output.TLS == nil {
			output.TLS = &telemetryv1beta1.OutputTLS{}
		}

		output.TLS.Insecure = insecure
	}
}

func OTLPInsecureSkipVerify(insecure bool) OTLPOutputOption {
	return func(output *telemetryv1beta1.OTLPOutput) {
		if output.TLS == nil {
			output.TLS = &telemetryv1beta1.OutputTLS{}
		}

		output.TLS.InsecureSkipVerify = insecure
	}
}

func OTLPProtocol(protocol telemetryv1beta1.OTLPProtocol) OTLPOutputOption {
	return func(output *telemetryv1beta1.OTLPOutput) {
		output.Protocol = protocol
	}
}

func OTLPEndpointPath(path string) OTLPOutputOption {
	return func(output *telemetryv1beta1.OTLPOutput) {
		output.Path = path
	}
}

type OAuth2Option func(oauth2 *telemetryv1beta1.OAuth2Options)

func OAuth2ClientID(clientID string) OAuth2Option {
	return func(oauth2 *telemetryv1beta1.OAuth2Options) {
		oauth2.ClientID = telemetryv1beta1.ValueType{Value: clientID}
	}
}

func OAuth2ClientIDFromSecret(secretName, secretNamespace, key string) OAuth2Option {
	return func(oauth2 *telemetryv1beta1.OAuth2Options) {
		oauth2.ClientID = telemetryv1beta1.ValueType{
			ValueFrom: &telemetryv1beta1.ValueFromSource{
				SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
					Name:      secretName,
					Namespace: secretNamespace,
					Key:       key,
				},
			},
		}
	}
}

func OAuth2ClientSecret(clientSecret string) OAuth2Option {
	return func(oauth2 *telemetryv1beta1.OAuth2Options) {
		oauth2.ClientSecret = telemetryv1beta1.ValueType{Value: clientSecret}
	}
}

func OAuth2ClientSecretFromSecret(secretName, secretNamespace, key string) OAuth2Option {
	return func(oauth2 *telemetryv1beta1.OAuth2Options) {
		oauth2.ClientSecret = telemetryv1beta1.ValueType{
			ValueFrom: &telemetryv1beta1.ValueFromSource{
				SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
					Name:      secretName,
					Namespace: secretNamespace,
					Key:       key,
				},
			},
		}
	}
}

func OAuth2TokenURL(tokenURL string) OAuth2Option {
	return func(oauth2 *telemetryv1beta1.OAuth2Options) {
		oauth2.TokenURL = telemetryv1beta1.ValueType{Value: tokenURL}
	}
}

func OAuth2TokenURLFromSecret(secretName, secretNamespace, key string) OAuth2Option {
	return func(oauth2 *telemetryv1beta1.OAuth2Options) {
		oauth2.TokenURL = telemetryv1beta1.ValueType{
			ValueFrom: &telemetryv1beta1.ValueFromSource{
				SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
					Name:      secretName,
					Namespace: secretNamespace,
					Key:       key,
				},
			},
		}
	}
}

func OAuth2Scopes(scopes []string) OAuth2Option {
	return func(oauth2 *telemetryv1beta1.OAuth2Options) {
		oauth2.Scopes = scopes
	}
}

func OAuth2Params(params map[string]string) OAuth2Option {
	return func(oauth2 *telemetryv1beta1.OAuth2Options) {
		oauth2.Params = params
	}
}

type HTTPOutputOption func(output *telemetryv1beta1.FluentBitHTTPOutput)

func HTTPClientTLSFromString(ca, cert, key string) HTTPOutputOption {
	return func(output *telemetryv1beta1.FluentBitHTTPOutput) {
		output.TLS = telemetryv1beta1.OutputTLS{
			CA:   &telemetryv1beta1.ValueType{Value: ca},
			Cert: &telemetryv1beta1.ValueType{Value: cert},
			Key:  &telemetryv1beta1.ValueType{Value: key},
		}
	}
}

func HTTPClientTLS(tls telemetryv1beta1.OutputTLS) HTTPOutputOption {
	return func(output *telemetryv1beta1.FluentBitHTTPOutput) {
		output.TLS = tls
	}
}

func HTTPHost(host string) HTTPOutputOption {
	return func(output *telemetryv1beta1.FluentBitHTTPOutput) {
		output.Host = telemetryv1beta1.ValueType{Value: host}
	}
}

func HTTPHostFromSecret(secretName, secretNamespace, key string) HTTPOutputOption {
	return func(output *telemetryv1beta1.FluentBitHTTPOutput) {
		output.Host = telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
			Name:      secretName,
			Namespace: secretNamespace,
			Key:       key,
		}}}
	}
}

func HTTPPort(port int32) HTTPOutputOption {
	return func(output *telemetryv1beta1.FluentBitHTTPOutput) {
		output.Port = strconv.Itoa(int(port))
	}
}

func HTTPDedot(dedot bool) HTTPOutputOption {
	return func(output *telemetryv1beta1.FluentBitHTTPOutput) {
		output.Dedot = dedot
	}
}

func HTTPBasicAuthFromSecret(secretName, secretNamespace, userKey, passwordKey string) HTTPOutputOption {
	return func(output *telemetryv1beta1.FluentBitHTTPOutput) {
		output.User = &telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
			Name:      secretName,
			Namespace: secretNamespace,
			Key:       userKey,
		}}}

		output.Password = &telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
			Name:      secretName,
			Namespace: secretNamespace,
			Key:       passwordKey,
		}}}
	}
}

type NamespaceSelectorOptions func(selector *telemetryv1beta1.NamespaceSelector)

func IncludeNamespaces(namespaces ...string) NamespaceSelectorOptions {
	return func(selector *telemetryv1beta1.NamespaceSelector) {
		selector.Include = namespaces
	}
}

func ExcludeNamespaces(namespaces ...string) NamespaceSelectorOptions {
	return func(selector *telemetryv1beta1.NamespaceSelector) {
		selector.Exclude = namespaces
	}
}
