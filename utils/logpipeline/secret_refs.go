package logpipeline

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	utils "github.com/kyma-project/telemetry-manager/utils/shared"
)

func GetSecretRefs(lp *telemetryv1alpha1.LogPipeline) []telemetryv1alpha1.SecretKeyRef {
	var refs []telemetryv1alpha1.SecretKeyRef

	for _, v := range lp.Spec.Variables {
		if v.ValueFrom.SecretKeyRef != nil {
			refs = append(refs, *v.ValueFrom.SecretKeyRef)
		}
	}

	refs = append(refs, GetEnvSecretRefs(lp)...)
	refs = append(refs, getTLSSecretRefs(lp)...)

	return refs
}

// GetEnvSecretRefs returns the secret references of a LogPipeline that should be stored in the env secret
func GetEnvSecretRefs(lp *telemetryv1alpha1.LogPipeline) []telemetryv1alpha1.SecretKeyRef {
	var refs []telemetryv1alpha1.SecretKeyRef

	output := lp.Spec.Output
	if output.HTTP != nil {
		refs = utils.AppendIfSecretRef(refs, output.HTTP.Host)
		refs = utils.AppendIfSecretRef(refs, output.HTTP.User)
		refs = utils.AppendIfSecretRef(refs, output.HTTP.Password)
	}

	return refs
}

func getTLSSecretRefs(lp *telemetryv1alpha1.LogPipeline) []telemetryv1alpha1.SecretKeyRef {
	var refs []telemetryv1alpha1.SecretKeyRef

	output := lp.Spec.Output
	if output.HTTP != nil {
		tlsConfig := output.HTTP.TLS
		if tlsConfig.CA != nil {
			refs = utils.AppendIfSecretRef(refs, *tlsConfig.CA)
		}

		if tlsConfig.Cert != nil {
			refs = utils.AppendIfSecretRef(refs, *tlsConfig.Cert)
		}

		if tlsConfig.Key != nil {
			refs = utils.AppendIfSecretRef(refs, *tlsConfig.Key)
		}
	}

	return refs
}
