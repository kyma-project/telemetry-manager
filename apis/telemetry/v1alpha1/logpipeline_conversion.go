package v1alpha1

import (
	"errors"

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

var errSrcTypeUnsupported = errors.New("source type is not LogPipeline v1alpha1")
var errDstTypeUnsupported = errors.New("destination type is not LogPipeline v1beta1")

// ConvertTo converts this LogPipeline to the Hub version (v1beta1).
func (lp *LogPipeline) ConvertTo(dstRaw conversion.Hub) error {
	src := lp
	dst, ok := dstRaw.(*telemetryv1beta1.LogPipeline)
	if !ok {
		return errDstTypeUnsupported
	}

	dst.ObjectMeta = src.ObjectMeta

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

	if srcHTTPOutput := src.Spec.Output.HTTP; srcHTTPOutput != nil {
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

	if srcCustomOutput := src.Spec.Output.Custom; srcCustomOutput != "" {
		dst.Spec.Output.Custom = srcCustomOutput
	}

	dst.Status = telemetryv1beta1.LogPipelineStatus(src.Status)

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

// ConvertFrom converts from the Hub version (v1beta1) to this version.
//
//lint:ignore ST1016 This is a conversion function and "dst" makes sense here.
func (lp *LogPipeline) ConvertFrom(srcRaw conversion.Hub) error {
	dst := lp
	src, ok := srcRaw.(*telemetryv1beta1.LogPipeline)
	if !ok {
		return errSrcTypeUnsupported
	}

	dst.ObjectMeta = src.ObjectMeta

	if srcAppInput := src.Spec.Input.Runtime; srcAppInput != nil {
		dst.Spec.Input.Application = ApplicationInput{
			Namespaces:       InputNamespaces(srcAppInput.Namespaces),
			Containers:       InputContainers(srcAppInput.Containers),
			KeepAnnotations:  srcAppInput.KeepAnnotations,
			DropLabels:       srcAppInput.DropLabels,
			KeepOriginalBody: srcAppInput.KeepOriginalBody,
		}
	}

	for _, f := range src.Spec.Files {
		dst.Spec.Files = append(dst.Spec.Files, FileMount(f))
	}

	for _, f := range src.Spec.Filters {
		dst.Spec.Filters = append(dst.Spec.Filters, Filter(f))
	}

	if srcHTTPOutput := src.Spec.Output.HTTP; srcHTTPOutput != nil {
		dst.Spec.Output.HTTP = &HTTPOutput{
			Host:      v1Beta1ValueTypeToV1Alpha1(srcHTTPOutput.Host),
			User:      v1Beta1ValueTypeToV1Alpha1(srcHTTPOutput.User),
			Password:  v1Beta1ValueTypeToV1Alpha1(srcHTTPOutput.Password),
			URI:       srcHTTPOutput.URI,
			Port:      srcHTTPOutput.Port,
			Compress:  srcHTTPOutput.Compress,
			Format:    srcHTTPOutput.Format,
			TLSConfig: v1Beta1TLSToV1Alpha1(srcHTTPOutput.TLSConfig),
			Dedot:     srcHTTPOutput.Dedot,
		}
	}

	if srcCustomOutput := src.Spec.Output.Custom; srcCustomOutput != "" {
		dst.Spec.Output.Custom = srcCustomOutput
	}

	dst.Status = LogPipelineStatus(src.Status)

	return nil
}

func v1Beta1TLSToV1Alpha1(src telemetryv1beta1.LogPipelineHTTPOutputTLS) TLSConfig {
	var dst TLSConfig
	if src.CA != nil {
		ca := v1Beta1ValueTypeToV1Alpha1(*src.CA)
		dst.CA = &ca
	}
	if src.Cert != nil {
		cert := v1Beta1ValueTypeToV1Alpha1(*src.Cert)
		dst.Cert = &cert
	}
	if src.Key != nil {
		key := v1Beta1ValueTypeToV1Alpha1(*src.Key)
		dst.Key = &key
	}
	dst.Disabled = src.Disabled
	dst.SkipCertificateValidation = src.SkipCertificateValidation
	return dst
}

func v1Beta1ValueTypeToV1Alpha1(src telemetryv1beta1.ValueType) ValueType {
	if src.ValueFrom != nil && src.ValueFrom.SecretKeyRef != nil {
		return ValueType{
			ValueFrom: &ValueFromSource{
				SecretKeyRef: (*SecretKeyRef)(src.ValueFrom.SecretKeyRef),
			},
		}
	}
	return ValueType{
		Value: src.Value,
	}
}
