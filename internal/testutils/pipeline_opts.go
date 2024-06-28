package testutils

import (
	"strconv"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type OTLPOutputOption func(*telemetryv1alpha1.OtlpOutput)

func OTLPEndpoint(endpoint string) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OtlpOutput) {
		output.Endpoint = telemetryv1alpha1.ValueType{Value: endpoint}
	}
}

func OTLPEndpointFromSecret(secretName, secretNamespace, endpointKey string) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OtlpOutput) {
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
	return func(output *telemetryv1alpha1.OtlpOutput) {
		output.Authentication = &telemetryv1alpha1.AuthenticationOptions{
			Basic: &telemetryv1alpha1.BasicAuthOptions{
				User:     telemetryv1alpha1.ValueType{Value: user},
				Password: telemetryv1alpha1.ValueType{Value: password},
			},
		}
	}
}

func OTLPBasicAuthFromSecret(secretName, secretNamespace, userKey, passwordKey string) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OtlpOutput) {
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
	return func(output *telemetryv1alpha1.OtlpOutput) {
		output.Headers = append(output.Headers, telemetryv1alpha1.Header{
			Name: name,
			ValueType: telemetryv1alpha1.ValueType{
				Value: value,
			},
			Prefix: prefix,
		})
	}
}

func OTLPClientTLS(ca, cert, key string) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OtlpOutput) {
		output.TLS = &telemetryv1alpha1.OtlpTLS{
			CA:   &telemetryv1alpha1.ValueType{Value: ca},
			Cert: &telemetryv1alpha1.ValueType{Value: cert},
			Key:  &telemetryv1alpha1.ValueType{Value: key},
		}
	}
}

func OTLPClientTLSMissingCA(cert, key string) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OtlpOutput) {
		output.TLS = &telemetryv1alpha1.OtlpTLS{
			Cert: &telemetryv1alpha1.ValueType{Value: cert},
			Key:  &telemetryv1alpha1.ValueType{Value: key},
		}
	}
}

func OTLPClientTLSMissingCert(ca, key string) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OtlpOutput) {
		output.TLS = &telemetryv1alpha1.OtlpTLS{
			CA:  &telemetryv1alpha1.ValueType{Value: ca},
			Key: &telemetryv1alpha1.ValueType{Value: key},
		}
	}
}

func OTLPClientTLSMissingKey(ca, cert string) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OtlpOutput) {
		output.TLS = &telemetryv1alpha1.OtlpTLS{
			CA:   &telemetryv1alpha1.ValueType{Value: ca},
			Cert: &telemetryv1alpha1.ValueType{Value: cert},
		}
	}
}

func OTLPClientTLSMissingAll() OTLPOutputOption {
	return func(output *telemetryv1alpha1.OtlpOutput) {
		output.TLS = &telemetryv1alpha1.OtlpTLS{
			Insecure:           true,
			InsecureSkipVerify: true,
		}
	}
}

func OTLPClientTLSMissingAllButCA(ca string) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OtlpOutput) {
		output.TLS = &telemetryv1alpha1.OtlpTLS{
			CA: &telemetryv1alpha1.ValueType{Value: ca},
		}
	}
}

func OTLPProtocol(protocol string) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OtlpOutput) {
		output.Protocol = protocol
	}
}

func OTLPEndpointPath(path string) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OtlpOutput) {
		output.Path = path
	}
}

type HTTPOutputOption func(output *telemetryv1alpha1.HTTPOutput)

func HTTPClientTLS(ca, cert, key string) HTTPOutputOption {
	return func(output *telemetryv1alpha1.HTTPOutput) {
		output.TLSConfig = telemetryv1alpha1.TLSConfig{
			CA:   &telemetryv1alpha1.ValueType{Value: ca},
			Cert: &telemetryv1alpha1.ValueType{Value: cert},
			Key:  &telemetryv1alpha1.ValueType{Value: key},
		}
	}
}

func HTTPClientTLSMissingCA(cert, key string) HTTPOutputOption {
	return func(output *telemetryv1alpha1.HTTPOutput) {
		output.TLSConfig = telemetryv1alpha1.TLSConfig{
			Cert: &telemetryv1alpha1.ValueType{Value: cert},
			Key:  &telemetryv1alpha1.ValueType{Value: key},
		}
	}
}

func HTTPClientTLSMissingCert(ca, key string) HTTPOutputOption {
	return func(output *telemetryv1alpha1.HTTPOutput) {
		output.TLSConfig = telemetryv1alpha1.TLSConfig{
			CA:  &telemetryv1alpha1.ValueType{Value: ca},
			Key: &telemetryv1alpha1.ValueType{Value: key},
		}
	}
}

func HTTPClientTLSMissingKey(ca, cert string) HTTPOutputOption {
	return func(output *telemetryv1alpha1.HTTPOutput) {
		output.TLSConfig = telemetryv1alpha1.TLSConfig{
			CA:   &telemetryv1alpha1.ValueType{Value: ca},
			Cert: &telemetryv1alpha1.ValueType{Value: cert},
		}
	}
}

func HTTPClientTLSMissingAll() HTTPOutputOption {
	return func(output *telemetryv1alpha1.HTTPOutput) {
		output.TLSConfig = telemetryv1alpha1.TLSConfig{
			Disabled:                  true,
			SkipCertificateValidation: true,
		}
	}
}

func HTTPClientTLSMissingAllButCA(ca string) HTTPOutputOption {
	return func(output *telemetryv1alpha1.HTTPOutput) {
		output.TLSConfig = telemetryv1alpha1.TLSConfig{
			CA: &telemetryv1alpha1.ValueType{Value: ca},
		}
	}
}

func HTTPHost(host string) HTTPOutputOption {
	return func(output *telemetryv1alpha1.HTTPOutput) {
		output.Host = telemetryv1alpha1.ValueType{Value: host}
	}
}

func HTTPHostFromSecret(secretName, secretNamespace, key string) HTTPOutputOption {
	return func(output *telemetryv1alpha1.HTTPOutput) {
		output.Host = telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      secretName,
			Namespace: secretNamespace,
			Key:       key,
		}}}
	}
}

func HTTPPort(port int) HTTPOutputOption {
	return func(output *telemetryv1alpha1.HTTPOutput) {
		output.Port = strconv.Itoa(port)
	}
}

func HTTPDedot(dedot bool) HTTPOutputOption {
	return func(output *telemetryv1alpha1.HTTPOutput) {
		output.Dedot = dedot
	}
}
