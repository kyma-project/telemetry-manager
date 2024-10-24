package testutils

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

type HTTPOutputOption func(output *telemetryv1alpha1.LogPipelineHTTPOutput)

func HTTPClientTLSFromString(ca, cert, key string) HTTPOutputOption {
	return func(output *telemetryv1alpha1.LogPipelineHTTPOutput) {
		output.TLSConfig = telemetryv1alpha1.LogPipelineOutputTLS{
			CA:   &telemetryv1alpha1.ValueType{Value: ca},
			Cert: &telemetryv1alpha1.ValueType{Value: cert},
			Key:  &telemetryv1alpha1.ValueType{Value: key},
		}
	}
}

func HTTPClientTLS(tls telemetryv1alpha1.LogPipelineOutputTLS) HTTPOutputOption {
	return func(output *telemetryv1alpha1.LogPipelineHTTPOutput) {
		output.TLSConfig = tls
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
