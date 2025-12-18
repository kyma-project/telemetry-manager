package v1alpha1

import (
	"errors"
	"unsafe"

	apiconversion "k8s.io/apimachinery/pkg/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/namespaces"
)

// Converts between v1alpha1 and v1beta1 LogPipeline CRDs
// Major API changes which require specific conversion logic are:
// - input.application (v1alpha1) is renamed to input.runtime (v1beta1).
// - NamespaceSelector in input.runtime (v1beta1) is using the shared selector of input.otlp which lead to not having a 'System' boolean field anymore.
// - output.http in v1beta1 is using the shared TLS section of the output.otlp, leading to a rename of 'Disabled' field to 'Insecure' and 'SkipCertificateValidation' to 'InsecureSkipVerify'.
// - input.runtime namespaces and containers are now pointers in v1beta1, requiring nil checks during conversion.
// Additionally, changes were done in shared types which are documented in the related file.

var errSrcTypeUnsupportedLogPipeline = errors.New("source type is not LogPipeline v1alpha1")
var errDstTypeUnsupportedLogPipeline = errors.New("destination type is not LogPipeline v1beta1")

func (lp *LogPipeline) ConvertTo(dstRaw conversion.Hub) error {
	src := lp

	dst, ok := dstRaw.(*telemetryv1beta1.LogPipeline)
	if !ok {
		return errDstTypeUnsupportedLogPipeline
	}

	// Call the conversion-gen generated function
	return Convert_v1alpha1_LogPipeline_To_v1beta1_LogPipeline(src, dst, nil)
}

func (lp *LogPipeline) ConvertFrom(srcRaw conversion.Hub) error {
	dst := lp

	src, ok := srcRaw.(*telemetryv1beta1.LogPipeline)
	if !ok {
		return errSrcTypeUnsupportedLogPipeline
	}

	// Call the conversion-gen generated function
	return Convert_v1beta1_LogPipeline_To_v1alpha1_LogPipeline(src, dst, nil)
}

func Convert_v1alpha1_LogPipelineHTTPOutput_To_v1beta1_LogPipelineHTTPOutput(in *LogPipelineHTTPOutput, out *telemetryv1beta1.LogPipelineHTTPOutput, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha1_LogPipelineHTTPOutput_To_v1beta1_LogPipelineHTTPOutput(in, out, s); err != nil {
		return err
	}

	out.TLSConfig = telemetryv1beta1.OutputTLS{
		CA:                 (*telemetryv1beta1.ValueType)(unsafe.Pointer(in.TLS.CA)),
		Cert:               (*telemetryv1beta1.ValueType)(unsafe.Pointer(in.TLS.Cert)),
		Key:                (*telemetryv1beta1.ValueType)(unsafe.Pointer(in.TLS.Key)),
		Insecure:           in.TLS.Disabled,
		InsecureSkipVerify: in.TLS.SkipCertificateValidation,
	}

	return nil
}

func Convert_v1beta1_LogPipelineHTTPOutput_To_v1alpha1_LogPipelineHTTPOutput(in *telemetryv1beta1.LogPipelineHTTPOutput, out *LogPipelineHTTPOutput, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_LogPipelineHTTPOutput_To_v1alpha1_LogPipelineHTTPOutput(in, out, s); err != nil {
		return err
	}

	out.TLS = LogPipelineOutputTLS{
		CA:                        (*ValueType)(unsafe.Pointer(in.TLSConfig.CA)),
		Cert:                      (*ValueType)(unsafe.Pointer(in.TLSConfig.Cert)),
		Key:                       (*ValueType)(unsafe.Pointer(in.TLSConfig.Key)),
		Disabled:                  in.TLSConfig.Insecure,
		SkipCertificateValidation: in.TLSConfig.InsecureSkipVerify,
	}

	return nil
}

func Convert_v1alpha1_LogPipelineInput_To_v1beta1_LogPipelineInput(in *LogPipelineInput, out *telemetryv1beta1.LogPipelineInput, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha1_LogPipelineInput_To_v1beta1_LogPipelineInput(in, out, s); err != nil {
		return err
	}

	if in.Application == nil {
		return nil
	}

	out.Runtime = &telemetryv1beta1.LogPipelineRuntimeInput{
		Enabled:          in.Application.Enabled,
		KeepAnnotations:  in.Application.KeepAnnotations,
		DropLabels:       in.Application.DropLabels,
		KeepOriginalBody: in.Application.KeepOriginalBody,
	}

	var excludes []string
	if len(in.Application.Namespaces.Include) == 0 && len(in.Application.Namespaces.Exclude) == 0 && !in.Application.Namespaces.System {
		excludes = namespaces.System()
	} else {
		excludes = sanitizeNamespaceNames(in.Application.Namespaces.Exclude)
	}

	out.Runtime.Namespaces = &telemetryv1beta1.NamespaceSelector{
		Include: sanitizeNamespaceNames(in.Application.Namespaces.Include),
		Exclude: excludes,
	}

	out.Runtime.Containers = &telemetryv1beta1.LogPipelineContainerSelector{
		Include: in.Application.Containers.Include,
		Exclude: in.Application.Containers.Exclude,
	}

	return nil
}

func Convert_v1beta1_LogPipelineInput_To_v1alpha1_LogPipelineInput(in *telemetryv1beta1.LogPipelineInput, out *LogPipelineInput, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_LogPipelineInput_To_v1alpha1_LogPipelineInput(in, out, s); err != nil {
		return err
	}

	if in.Runtime == nil {
		return nil
	}

	out.Application = &LogPipelineApplicationInput{
		Enabled:          in.Runtime.Enabled,
		KeepAnnotations:  in.Runtime.KeepAnnotations,
		DropLabels:       in.Runtime.DropLabels,
		KeepOriginalBody: in.Runtime.KeepOriginalBody,
	}

	if in.Runtime.Namespaces != nil {
		out.Application.Namespaces = LogPipelineNamespaceSelector{
			Include: in.Runtime.Namespaces.Include,
			Exclude: in.Runtime.Namespaces.Exclude,
			System:  false,
		}
	}

	if in.Runtime.Containers != nil {
		out.Application.Containers = LogPipelineContainerSelector{
			Include: in.Runtime.Containers.Include,
			Exclude: in.Runtime.Containers.Exclude,
		}
	}

	return nil
}
