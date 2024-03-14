package v1beta1

func (lp *LogPipeline) GetSecretRefs() []SecretKeyRef {
	var refs []SecretKeyRef

	for _, v := range lp.Spec.Variables {
		if v.ValueFrom.IsSecretKeyRef() {
			refs = append(refs, *v.ValueFrom.SecretKeyRef)
		}
	}

	refs = append(refs, lp.GetEnvSecretRefs()...)
	refs = append(refs, lp.GetTLSSecretRefs()...)

	return refs
}

// GetEnvSecretRefs returns the secret references of a LogPipeline that should be stored in the env secret
func (lp *LogPipeline) GetEnvSecretRefs() []SecretKeyRef {
	var refs []SecretKeyRef

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

func (lp *LogPipeline) GetTLSSecretRefs() []SecretKeyRef {
	var refs []SecretKeyRef

	output := lp.Spec.Output
	if output.IsHTTPDefined() {
		tlsConfig := output.HTTP.TLSConfig
		if tlsConfig.CA != nil {
			refs = appendIfSecretRef(refs, *tlsConfig.CA)
		}
		if tlsConfig.Cert != nil {
			refs = appendIfSecretRef(refs, *tlsConfig.Cert)
		}
		if tlsConfig.Key != nil {
			refs = appendIfSecretRef(refs, *tlsConfig.Key)
		}
	}

	return refs
}

func (tp *TracePipeline) GetSecretRefs() []SecretKeyRef {
	return getRefsInOTLPOutput(tp.Spec.Output.OTLP)
}

func (mp *MetricPipeline) GetSecretRefs() []SecretKeyRef {
	return getRefsInOTLPOutput(mp.Spec.Output.OTLP)
}

func getRefsInOTLPOutput(OTLPOut *OTLPOutput) []SecretKeyRef {
	var refs []SecretKeyRef

	refs = appendIfSecretRef(refs, OTLPOut.Endpoint)

	if OTLPOut.Authentication != nil && OTLPOut.Authentication.Basic.IsDefined() {
		refs = appendIfSecretRef(refs, OTLPOut.Authentication.Basic.User)
		refs = appendIfSecretRef(refs, OTLPOut.Authentication.Basic.Password)
	}

	for _, header := range OTLPOut.Headers {
		refs = appendIfSecretRef(refs, header.ValueType)
	}

	if OTLPOut.TLS != nil && !OTLPOut.TLS.Insecure {
		if OTLPOut.TLS.CA != nil {
			refs = appendIfSecretRef(refs, *OTLPOut.TLS.CA)
		}
		if OTLPOut.TLS.Cert != nil {
			refs = appendIfSecretRef(refs, *OTLPOut.TLS.Cert)
		}
		if OTLPOut.TLS.Key != nil {
			refs = appendIfSecretRef(refs, *OTLPOut.TLS.Key)
		}
	}

	return refs
}

func appendIfSecretRef(secretKeyRefs []SecretKeyRef, valueType ValueType) []SecretKeyRef {
	if valueType.Value == "" && valueType.ValueFrom != nil && valueType.ValueFrom.IsSecretKeyRef() {
		secretKeyRefs = append(secretKeyRefs, *valueType.ValueFrom.SecretKeyRef)
	}
	return secretKeyRefs
}
