package testutils

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type OTLPOutputOption func(*telemetryv1alpha1.OtlpOutput)

func OTLPEndpoint(endpoint string) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OtlpOutput) {
		output.Endpoint = telemetryv1alpha1.ValueType{Value: endpoint}
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

func OTLPClientTLS(cert, key string) OTLPOutputOption {
	return func(output *telemetryv1alpha1.OtlpOutput) {
		output.TLS = &telemetryv1alpha1.OtlpTLS{
			Cert: &telemetryv1alpha1.ValueType{Value: cert},
			Key:  &telemetryv1alpha1.ValueType{Value: key},
		}
	}
}

type HTTPOutputOption func(output *telemetryv1alpha1.HTTPOutput)

func HTTPClientTLS(cert, key string) HTTPOutputOption {
	return func(output *telemetryv1alpha1.HTTPOutput) {
		output.TLSConfig = telemetryv1alpha1.TLSConfig{
			Cert: &telemetryv1alpha1.ValueType{Value: cert},
			Key:  &telemetryv1alpha1.ValueType{Value: key},
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
