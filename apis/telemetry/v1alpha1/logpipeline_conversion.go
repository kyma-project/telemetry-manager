package v1alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

// ConvertTo converts this CronJob to the Hub version (v1).
func (src *LogPipeline) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*telemetryv1beta1.LogPipeline)

	srcAppInput := src.Spec.Input.Application
	dst.Spec.Input = telemetryv1beta1.LogPipelineInput{
		Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
			Namespaces:       telemetryv1beta1.LogPipelineInputNamespaces(srcAppInput.Namespaces),
			Containers:       telemetryv1beta1.LogPipelineInputContainers(srcAppInput.Containers),
			KeepAnnotations:  srcAppInput.KeepAnnotations,
			DropLabels:       srcAppInput.DropLabels,
			KeepOriginalBody: srcAppInput.KeepOriginalBody,
		},
	}

	for _, f := range src.Spec.Files {
		dst.Spec.Files = append(dst.Spec.Files, telemetryv1beta1.LogPipelineFileMount(f))
	}

	for _, f := range src.Spec.Filters {
		dst.Spec.Filters = append(dst.Spec.Filters, telemetryv1beta1.LogPipelineFilter(f))
	}

	srcHTTPOutput := src.Spec.Output.HTTP
	if srcHTTPOutput != nil {
		dst.Spec.Output.HTTP = &telemetryv1beta1.LogPipelineHTTPOutput{
			Host:      v1Alpha1ValueTypeToV1Beta1(srcHTTPOutput.Host),
			User:      v1Alpha1ValueTypeToV1Beta1(srcHTTPOutput.User),
			Password:  v1Alpha1ValueTypeToV1Beta1(srcHTTPOutput.Password),
			URI:       srcHTTPOutput.URI,
			Port:      srcHTTPOutput.Port,
			Compress:  srcHTTPOutput.Compress,
			Format:    srcHTTPOutput.Format,
			TLSConfig: v1Alpha1TLSToV1Beta1(srcHTTPOutput.TLSConfig),
			Dedot:     srcHTTPOutput.Dedot,
		}
	}

	srcCustomOutput := src.Spec.Output.Custom
	if srcCustomOutput != "" {
		dst.Spec.Output.Custom = srcCustomOutput
	}

	return nil
}

func v1Alpha1ValueTypeToV1Beta1(src ValueType) telemetryv1beta1.ValueType {
	if src.ValueFrom != nil && src.ValueFrom.SecretKeyRef != nil {
		return telemetryv1beta1.ValueType{
			ValueFrom: &telemetryv1beta1.ValueFromSource{
				SecretKeyRef: (*telemetryv1beta1.SecretKeyRef)(src.ValueFrom.SecretKeyRef),
			},
		}
	}
	return telemetryv1beta1.ValueType{
		Value: src.Value,
	}
}

func v1Alpha1TLSToV1Beta1(src TLSConfig) telemetryv1beta1.LogPipelineHTTPOutputTLS {
	var dst telemetryv1beta1.LogPipelineHTTPOutputTLS
	if src.CA != nil {
		ca := v1Alpha1ValueTypeToV1Beta1(*src.CA)
		dst.CA = &ca
	}
	if src.Cert != nil {
		cert := v1Alpha1ValueTypeToV1Beta1(*src.Cert)
		dst.Cert = &cert
	}
	if src.Key != nil {
		key := v1Alpha1ValueTypeToV1Beta1(*src.Key)
		dst.Key = &key
	}
	dst.Disabled = src.Disabled
	dst.SkipCertificateValidation = src.SkipCertificateValidation

	return dst
}
