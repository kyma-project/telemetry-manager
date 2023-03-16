package v1alpha1

import (
	"github.com/kyma-project/telemetry-manager/internal/field"
	"github.com/kyma-project/telemetry-manager/internal/utils/envvar"
)

func (lp *LogPipeline) GetSecretRefs() []field.Descriptor {
	var fields []field.Descriptor

	for _, v := range lp.Spec.Variables {
		if !v.ValueFrom.IsSecretKeyRef() {
			continue
		}

		fields = append(fields, field.Descriptor{
			TargetSecretKey:       v.Name,
			SourceSecretName:      v.ValueFrom.SecretKeyRef.Name,
			SourceSecretNamespace: v.ValueFrom.SecretKeyRef.Namespace,
			SourceSecretKey:       v.ValueFrom.SecretKeyRef.Key,
		})
	}

	output := lp.Spec.Output
	if output.IsHTTPDefined() {
		fields = appendIfSecretRef(fields, lp.Name, output.HTTP.Host)
		fields = appendIfSecretRef(fields, lp.Name, output.HTTP.User)
		fields = appendIfSecretRef(fields, lp.Name, output.HTTP.Password)
	}
	if output.IsLokiDefined() {
		fields = appendIfSecretRef(fields, lp.Name, output.Loki.URL)
	}

	return fields
}

func (tp *TracePipeline) GetSecretRefs() []field.Descriptor {
	return getRefsInOtlpOutput(tp.Spec.Output.Otlp, tp.Name)
}

func (mp *MetricPipeline) GetSecretRefs() []field.Descriptor {
	return getRefsInOtlpOutput(mp.Spec.Output.Otlp, mp.Name)
}

func getRefsInOtlpOutput(otlpOut *OtlpOutput, pipelineName string) []field.Descriptor {
	var result []field.Descriptor

	if otlpOut.Endpoint.ValueFrom != nil && otlpOut.Endpoint.ValueFrom.IsSecretKeyRef() {
		result = append(result, field.Descriptor{
			TargetSecretKey:       "OTLP_ENDPOINT",
			SourceSecretName:      otlpOut.Endpoint.ValueFrom.SecretKeyRef.Name,
			SourceSecretNamespace: otlpOut.Endpoint.ValueFrom.SecretKeyRef.Namespace,
			SourceSecretKey:       otlpOut.Endpoint.ValueFrom.SecretKeyRef.Key,
		})
	}

	if otlpOut.Authentication != nil && otlpOut.Authentication.Basic.IsDefined() {
		result = appendIfSecretRef(result, pipelineName, otlpOut.Authentication.Basic.User)
		result = appendIfSecretRef(result, pipelineName, otlpOut.Authentication.Basic.Password)
	}

	// TODO test header
	for _, header := range otlpOut.Headers {
		result = appendIfSecretRef(result, pipelineName, header.ValueType)
	}

	return result
}

func appendIfSecretRef(fields []field.Descriptor, pipelineName string, valueType ValueType) []field.Descriptor {
	if valueType.Value == "" && valueType.ValueFrom != nil && valueType.ValueFrom.IsSecretKeyRef() {
		secretKeyRef := *valueType.ValueFrom.SecretKeyRef
		fields = append(fields, field.Descriptor{
			TargetSecretKey:       envvar.FormatEnvVarName(pipelineName, secretKeyRef.Namespace, secretKeyRef.Name, secretKeyRef.Key),
			SourceSecretName:      valueType.ValueFrom.SecretKeyRef.Name,
			SourceSecretNamespace: valueType.ValueFrom.SecretKeyRef.Namespace,
			SourceSecretKey:       valueType.ValueFrom.SecretKeyRef.Key,
		})
	}
	return fields
}
