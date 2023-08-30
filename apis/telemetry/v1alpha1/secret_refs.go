package v1alpha1

func (lp *LogPipeline) GetSecretRefs() []SecretKeyRef {
	var refs []SecretKeyRef

	for _, v := range lp.Spec.Variables {
		if !v.ValueFrom.IsSecretKeyRef() {
			continue
		}

		refs = append(refs, *v.ValueFrom.SecretKeyRef)
	}

	output := lp.Spec.Output
	if output.IsHTTPDefined() {
		refs = appendIfSecretRef(refs, output.HTTP.Host)
		refs = appendIfSecretRef(refs, output.HTTP.User)
		refs = appendIfSecretRef(refs, output.HTTP.Password)
	}
	if output.IsLokiDefined() {
		refs = appendIfSecretRef(refs, output.Loki.URL)
	}

	return refs
}

func (tp *TracePipeline) GetSecretRefs() []SecretKeyRef {
	return getRefsInOtlpOutput(tp.Spec.Output.Otlp)
}

func (mp *MetricPipeline) GetSecretRefs() []SecretKeyRef {
	return getRefsInOtlpOutput(mp.Spec.Output.Otlp)
}

func getRefsInOtlpOutput(otlpOut *OtlpOutput) []SecretKeyRef {
	var refs []SecretKeyRef

	refs = appendIfSecretRef(refs, otlpOut.Endpoint)

	if otlpOut.Authentication != nil && otlpOut.Authentication.Basic.IsDefined() {
		refs = appendIfSecretRef(refs, otlpOut.Authentication.Basic.User)
		refs = appendIfSecretRef(refs, otlpOut.Authentication.Basic.Password)
	}

	for _, header := range otlpOut.Headers {
		refs = appendIfSecretRef(refs, header.ValueType)
	}

	if otlpOut.TLS != nil && !otlpOut.TLS.Insecure {
		refs = appendIfSecretRef(refs, otlpOut.TLS.Cert)
		refs = appendIfSecretRef(refs, otlpOut.TLS.Key)
		refs = appendIfSecretRef(refs, otlpOut.TLS.CA)
	}

	return refs
}

func appendIfSecretRef(secretKeyRefs []SecretKeyRef, valueType ValueType) []SecretKeyRef {
	if valueType.Value == "" && valueType.ValueFrom != nil && valueType.ValueFrom.IsSecretKeyRef() {
		secretKeyRefs = append(secretKeyRefs, *valueType.ValueFrom.SecretKeyRef)
	}
	return secretKeyRefs
}
