package shared

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func AppendIfSecretRef(secretKeyRefs []telemetryv1alpha1.SecretKeyRef, valueType telemetryv1alpha1.ValueType) []telemetryv1alpha1.SecretKeyRef {
	if valueType.Value == "" && valueType.ValueFrom != nil && valueType.ValueFrom.SecretKeyRef != nil {
		secretKeyRefs = append(secretKeyRefs, *valueType.ValueFrom.SecretKeyRef)
	}

	return secretKeyRefs
}

func GetRefsInOTLPOutput(otlpOut *telemetryv1alpha1.OTLPOutput) []telemetryv1alpha1.SecretKeyRef {
	var refs []telemetryv1alpha1.SecretKeyRef

	refs = AppendIfSecretRef(refs, otlpOut.Endpoint)

	if otlpOut.Authentication != nil && otlpOut.Authentication.Basic != nil {
		refs = AppendIfSecretRef(refs, otlpOut.Authentication.Basic.User)
		refs = AppendIfSecretRef(refs, otlpOut.Authentication.Basic.Password)
	}

	for _, header := range otlpOut.Headers {
		refs = AppendIfSecretRef(refs, header.ValueType)
	}

	if otlpOut.TLS != nil && !otlpOut.TLS.Insecure {
		if otlpOut.TLS.CA != nil {
			refs = AppendIfSecretRef(refs, *otlpOut.TLS.CA)
		}

		if otlpOut.TLS.Cert != nil {
			refs = AppendIfSecretRef(refs, *otlpOut.TLS.Cert)
		}

		if otlpOut.TLS.Key != nil {
			refs = AppendIfSecretRef(refs, *otlpOut.TLS.Key)
		}
	}

	return refs
}
