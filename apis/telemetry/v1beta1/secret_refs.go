package v1beta1

func (lp *LogPipeline) GetSecretRefs() []SecretKeyRef {
	var refs []SecretKeyRef

	for _, v := range lp.Spec.Variables {
		if v.ValueFrom.SecretKeyRef != nil {
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
	if output.HTTP != nil {
		refs = appendIfSecretRef(refs, output.HTTP.Host)
		refs = appendIfSecretRef(refs, output.HTTP.User)
		refs = appendIfSecretRef(refs, output.HTTP.Password)
	}

	return refs
}

func (lp *LogPipeline) GetTLSSecretRefs() []SecretKeyRef {
	var refs []SecretKeyRef

	output := lp.Spec.Output
	if output.HTTP != nil {
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

func getRefsInOTLPOutput(out *OTLPOutput) []SecretKeyRef {
	var refs []SecretKeyRef

	refs = appendIfSecretRef(refs, out.Endpoint)

	if out.Authentication != nil && out.Authentication.Basic != nil {
		refs = appendIfSecretRef(refs, out.Authentication.Basic.User)
		refs = appendIfSecretRef(refs, out.Authentication.Basic.Password)
	}

	for _, header := range out.Headers {
		refs = appendIfSecretRef(refs, header.ValueType)
	}

	if out.TLS != nil && !out.TLS.Disabled {
		if out.TLS.CA != nil {
			refs = appendIfSecretRef(refs, *out.TLS.CA)
		}

		if out.TLS.Cert != nil {
			refs = appendIfSecretRef(refs, *out.TLS.Cert)
		}

		if out.TLS.Key != nil {
			refs = appendIfSecretRef(refs, *out.TLS.Key)
		}
	}

	return refs
}

func appendIfSecretRef(secretKeyRefs []SecretKeyRef, valueType ValueType) []SecretKeyRef {
	if valueType.Value == "" && valueType.ValueFrom != nil && valueType.ValueFrom.SecretKeyRef != nil {
		secretKeyRefs = append(secretKeyRefs, *valueType.ValueFrom.SecretKeyRef)
	}

	return secretKeyRefs
}
