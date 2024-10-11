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
		Runtime: telemetryv1beta1.LogPipelineRuntimeInput{
			Enabled:          srcAppInput.Enabled,
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

	if srcOTLPOutput := src.Spec.Output.Otlp; srcOTLPOutput != nil {
		dst.Spec.Output.OTLP = &telemetryv1beta1.OTLPOutput{
			Protocol:       telemetryv1beta1.OTLPProtocol(srcOTLPOutput.Protocol),
			Endpoint:       v1Alpha1ValueTypeToV1Beta1(srcOTLPOutput.Endpoint),
			Path:           srcOTLPOutput.Path,
			Authentication: v1Alpha1AuthenticationToV1Beta1(srcOTLPOutput.Authentication),
			Headers:        v1Alpha1HeadersToV1Beta1(srcOTLPOutput.Headers),
			TLS:            v1Alpha1OtlpTLSToV1Beta1(srcOTLPOutput.TLS),
		}
	}

	if srcCustomOutput := src.Spec.Output.Custom; srcCustomOutput != "" {
		dst.Spec.Output.Custom = srcCustomOutput
	}

	dst.Status = telemetryv1beta1.LogPipelineStatus(src.Status)

	return nil
}

func v1Alpha1OtlpTLSToV1Beta1(tls *OtlpTLS) *telemetryv1beta1.OutputTLS {
	if tls == nil {
		return nil
	}

	betaTLS := &telemetryv1beta1.OutputTLS{
		Disabled:                  tls.Insecure,
		SkipCertificateValidation: tls.InsecureSkipVerify,
	}

	if tls.CA != nil {
		ca := v1Alpha1ValueTypeToV1Beta1(*tls.CA)
		betaTLS.CA = &ca
	}
	if tls.Key != nil {
		key := v1Alpha1ValueTypeToV1Beta1(*tls.Key)
		betaTLS.Key = &key
	}
	if tls.Cert != nil {
		cert := v1Alpha1ValueTypeToV1Beta1(*tls.Cert)
		betaTLS.Cert = &cert
	}
	return betaTLS
}

func v1Alpha1HeadersToV1Beta1(headers []Header) []telemetryv1beta1.Header {
	var dst []telemetryv1beta1.Header
	for _, h := range headers {
		dst = append(dst, v1Alpha1HeaderToV1Beta1(h))
	}
	return dst

}

func v1Alpha1HeaderToV1Beta1(h Header) telemetryv1beta1.Header {
	return telemetryv1beta1.Header{
		Name:      h.Name,
		ValueType: v1Alpha1ValueTypeToV1Beta1(h.ValueType),
		Prefix:    h.Prefix,
	}
}

func v1Alpha1AuthenticationToV1Beta1(authentication *AuthenticationOptions) *telemetryv1beta1.AuthenticationOptions {
	if authentication == nil {
		return nil
	}
	return &telemetryv1beta1.AuthenticationOptions{
		Basic: v1Alpha1BasicAuthOptionsToV1Beta1(authentication.Basic),
	}
}

func v1Alpha1BasicAuthOptionsToV1Beta1(basic *BasicAuthOptions) *telemetryv1beta1.BasicAuthOptions {
	if basic == nil {
		return nil
	}
	return &telemetryv1beta1.BasicAuthOptions{
		User:     v1Alpha1ValueTypeToV1Beta1(basic.User),
		Password: v1Alpha1ValueTypeToV1Beta1(basic.Password),
	}
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

func v1Alpha1TLSToV1Beta1(src TLSConfig) telemetryv1beta1.OutputTLS {
	var dst telemetryv1beta1.OutputTLS

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
func (lp *LogPipeline) ConvertFrom(srcRaw conversion.Hub) error {
	dst := lp
	src, ok := srcRaw.(*telemetryv1beta1.LogPipeline)
	if !ok {
		return errSrcTypeUnsupported
	}

	dst.ObjectMeta = src.ObjectMeta

	srcRuntimeInput := src.Spec.Input.Runtime
	dst.Spec.Input.Application = ApplicationInput{
		Enabled:          srcRuntimeInput.Enabled,
		Namespaces:       InputNamespaces(srcRuntimeInput.Namespaces),
		Containers:       InputContainers(srcRuntimeInput.Containers),
		KeepAnnotations:  srcRuntimeInput.KeepAnnotations,
		DropLabels:       srcRuntimeInput.DropLabels,
		KeepOriginalBody: srcRuntimeInput.KeepOriginalBody,
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

	if srcOTLPOutput := src.Spec.Output.OTLP; srcOTLPOutput != nil {
		dst.Spec.Output.Otlp = &OtlpOutput{
			Protocol:       (string)(srcOTLPOutput.Protocol),
			Endpoint:       v1Beta1ValueTypeToV1Alpha1(srcOTLPOutput.Endpoint),
			Path:           srcOTLPOutput.Path,
			Authentication: v1Beta1AuthenticationToV1Alpha1(srcOTLPOutput.Authentication),
			Headers:        v1Beta1HeadersToV1Alpha1(srcOTLPOutput.Headers),
			TLS:            v1Beta1OtlpTLSToV1Alpha1(srcOTLPOutput.TLS),
		}
	}

	if srcCustomOutput := src.Spec.Output.Custom; srcCustomOutput != "" {
		dst.Spec.Output.Custom = srcCustomOutput
	}

	dst.Status = LogPipelineStatus(src.Status)

	return nil
}

func v1Beta1OtlpTLSToV1Alpha1(tls *telemetryv1beta1.OutputTLS) *OtlpTLS {
	if tls == nil {
		return nil
	}

	alphaTLS := &OtlpTLS{
		Insecure:           tls.Disabled,
		InsecureSkipVerify: tls.SkipCertificateValidation,
	}

	if tls.CA != nil {
		ca := v1Beta1ValueTypeToV1Alpha1(*tls.CA)
		alphaTLS.CA = &ca
	}
	if tls.Key != nil {
		key := v1Beta1ValueTypeToV1Alpha1(*tls.Key)
		alphaTLS.Key = &key
	}
	if tls.Cert != nil {
		cert := v1Beta1ValueTypeToV1Alpha1(*tls.Cert)
		alphaTLS.Cert = &cert
	}
	return alphaTLS
}

func v1Beta1HeadersToV1Alpha1(headers []telemetryv1beta1.Header) []Header {
	var dst []Header
	for _, h := range headers {
		dst = append(dst, v1Beta1HeaderToV1Alpha1(h))
	}
	return dst
}

func v1Beta1HeaderToV1Alpha1(h telemetryv1beta1.Header) Header {
	return Header{
		Name:      h.Name,
		ValueType: v1Beta1ValueTypeToV1Alpha1(h.ValueType),
		Prefix:    h.Prefix,
	}
}

func v1Beta1AuthenticationToV1Alpha1(authentication *telemetryv1beta1.AuthenticationOptions) *AuthenticationOptions {
	if authentication == nil {
		return nil
	}
	return &AuthenticationOptions{
		Basic: v1Beta1BasicAuthOptionsToV1Alpha1(authentication.Basic),
	}
}

func v1Beta1BasicAuthOptionsToV1Alpha1(basic *telemetryv1beta1.BasicAuthOptions) *BasicAuthOptions {
	if basic == nil {
		return nil
	}
	return &BasicAuthOptions{
		User:     v1Beta1ValueTypeToV1Alpha1(basic.User),
		Password: v1Beta1ValueTypeToV1Alpha1(basic.Password),
	}
}

func v1Beta1TLSToV1Alpha1(src telemetryv1beta1.OutputTLS) TLSConfig {
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
