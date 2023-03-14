package v1alpha1

import (
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/field"
	"strings"
)

func (lp *LogPipeline) GetSecretRefs() []field.Descriptor {
	var fields []field.Descriptor

	for _, v := range lp.Spec.Variables {
		if !v.ValueFrom.IsSecretKeyRef() {
			continue
		}

		fields = append(fields, field.Descriptor{
			TargetSecretKey: v.Name,
			SecretKeyRef: field.SecretKeyRef{
				Name:      v.ValueFrom.SecretKeyRef.Name,
				Namespace: v.ValueFrom.SecretKeyRef.Namespace,
				Key:       v.ValueFrom.SecretKeyRef.Key,
			},
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
			TargetSecretKey: otlpOut.Endpoint.ValueFrom.SecretKeyRef.Name,
			SecretKeyRef: field.SecretKeyRef{
				Name:      otlpOut.Endpoint.ValueFrom.SecretKeyRef.Name,
				Namespace: otlpOut.Endpoint.ValueFrom.SecretKeyRef.Namespace,
				Key:       otlpOut.Endpoint.ValueFrom.SecretKeyRef.Key,
			},
		})
	}

	if otlpOut.Authentication != nil && otlpOut.Authentication.Basic.IsDefined() {
		result = appendIfSecretRef(result, pipelineName, otlpOut.Authentication.Basic.User)
		result = appendIfSecretRef(result, pipelineName, otlpOut.Authentication.Basic.Password)
	}

	for _, header := range otlpOut.Headers {
		result = appendIfSecretRef(result, pipelineName, header.ValueType)
	}

	return result
}

func appendIfSecretRef(fields []field.Descriptor, pipelineName string, valueType ValueType) []field.Descriptor {
	if valueType.Value == "" && valueType.ValueFrom != nil && valueType.ValueFrom.IsSecretKeyRef() {
		fields = append(fields, field.Descriptor{
			TargetSecretKey: formatTargetKey(pipelineName, *valueType.ValueFrom.SecretKeyRef),
			SecretKeyRef: field.SecretKeyRef{
				Name:      valueType.ValueFrom.SecretKeyRef.Name,
				Namespace: valueType.ValueFrom.SecretKeyRef.Namespace,
				Key:       valueType.ValueFrom.SecretKeyRef.Key,
			},
		})
	}

	return fields
}

func formatTargetKey(prefix string, secretKeyRef SecretKeyRef) string {
	result := fmt.Sprintf("%s_%s_%s_%s", prefix, secretKeyRef.Namespace, secretKeyRef.Name, secretKeyRef.Key)
	return makeEnvVarCompliant(result)
}

func makeEnvVarCompliant(input string) string {
	result := input
	result = strings.ToUpper(result)
	result = strings.Replace(result, ".", "_", -1)
	result = strings.Replace(result, "-", "_", -1)
	return result
}
