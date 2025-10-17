package v1alpha1

import (
	"errors"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

var errSrcTypeUnsupportedLogPipeline = errors.New("source type is not LogPipeline v1alpha1")
var errDstTypeUnsupportedLogPipeline = errors.New("destination type is not LogPipeline v1beta1")

// ConvertTo converts this LogPipeline to the Hub version (v1alpha1 -> v1beta1).
func (lp *LogPipeline) ConvertTo(dstRaw conversion.Hub) error {
	src := lp

	dst, ok := dstRaw.(*telemetryv1beta1.LogPipeline)
	if !ok {
		return errDstTypeUnsupportedLogPipeline
	}

	// Copy metadata
	dst.ObjectMeta = src.ObjectMeta

	// Copy Spec fields
	dst.Spec.Input = telemetryv1beta1.LogPipelineInput{}
	dst.Spec.Input.Runtime = convertApplicationToBeta(src.Spec.Input.Application)

	if src.Spec.Input.OTLP != nil {
		dst.Spec.Input.OTLP = &telemetryv1beta1.OTLPInput{
			Enabled:    ptr.To(!src.Spec.Input.OTLP.Disabled),
			Namespaces: convertNamespaceSelectorToBeta(src.Spec.Input.OTLP.Namespaces),
		}
	}

	for _, f := range src.Spec.Files {
		dst.Spec.Files = append(dst.Spec.Files, telemetryv1beta1.LogPipelineFileMount(f))
	}

	for _, f := range src.Spec.FluentBitFilters {
		dst.Spec.FluentBitFilters = append(dst.Spec.FluentBitFilters, telemetryv1beta1.LogPipelineFilter(f))
	}

	if srcHTTPOutput := src.Spec.Output.HTTP; srcHTTPOutput != nil {
		dst.Spec.Output.HTTP = &telemetryv1beta1.LogPipelineHTTPOutput{
			Host:      convertValueTypeToBeta(srcHTTPOutput.Host),
			URI:       srcHTTPOutput.URI,
			Port:      srcHTTPOutput.Port,
			Compress:  srcHTTPOutput.Compress,
			Format:    srcHTTPOutput.Format,
			TLSConfig: convertOutputTLSToBeta(srcHTTPOutput.TLS),
			Dedot:     srcHTTPOutput.Dedot,
		}

		if srcHTTPOutput.User != nil && (srcHTTPOutput.User.Value != "" || srcHTTPOutput.User.ValueFrom != nil) {
			user := convertValueTypeToBeta(*srcHTTPOutput.User)
			dst.Spec.Output.HTTP.User = &user
		}

		if srcHTTPOutput.Password != nil && (srcHTTPOutput.Password.Value != "" || srcHTTPOutput.Password.ValueFrom != nil) {
			password := convertValueTypeToBeta(*srcHTTPOutput.Password)
			dst.Spec.Output.HTTP.Password = &password
		}
	}

	if src.Spec.Output.OTLP != nil {
		dst.Spec.Output.OTLP = convertOTLPOutputToBeta(src.Spec.Output.OTLP)
	}

	if srcCustomOutput := src.Spec.Output.Custom; srcCustomOutput != "" {
		dst.Spec.Output.Custom = srcCustomOutput
	}

	if src.Spec.Transforms != nil {
		for _, t := range src.Spec.Transforms {
			dst.Spec.Transforms = append(dst.Spec.Transforms, convertTransformSpecToBeta(t))
		}
	}

	if src.Spec.Filters != nil {
		for _, t := range src.Spec.Filters {
			dst.Spec.Filters = append(dst.Spec.Filters, convertFilterSpecToBeta(t))
		}
	}

	dst.Status = telemetryv1beta1.LogPipelineStatus(src.Status)

	return nil
}

func convertApplicationToBeta(application *LogPipelineApplicationInput) *telemetryv1beta1.LogPipelineRuntimeInput {
	if application == nil {
		return nil
	}

	var excludes []string
	if len(application.Namespaces.Include) == 0 && len(application.Namespaces.Exclude) == 0 && !application.Namespaces.System {
		excludes = []string{"kyma-system", "kube-system", "istio-system", "compass-system"}
	} else {
		excludes = application.Namespaces.Exclude
	}

	runtime := &telemetryv1beta1.LogPipelineRuntimeInput{
		Enabled: application.Enabled,
		Namespaces: &telemetryv1beta1.NamespaceSelector{
			Include: sanitizeNamespaceNames(application.Namespaces.Include),
			Exclude: sanitizeNamespaceNames(excludes),
		},
		Containers: telemetryv1beta1.LogPipelineContainerSelector{
			Include: application.Containers.Include,
			Exclude: application.Containers.Exclude,
		},
		KeepAnnotations:  application.KeepAnnotations,
		DropLabels:       application.DropLabels,
		KeepOriginalBody: application.KeepOriginalBody,
	}

	return runtime
}

func convertOutputTLSToBeta(src LogPipelineOutputTLS) telemetryv1beta1.OutputTLS {
	var dst telemetryv1beta1.OutputTLS

	dst.CA = convertValueTypeToBetaPtr(src.CA)
	dst.Cert = convertValueTypeToBetaPtr(src.Cert)
	dst.Key = convertValueTypeToBetaPtr(src.Key)
	dst.Disabled = src.Disabled
	dst.SkipCertificateValidation = src.SkipCertificateValidation

	return dst
}

// ConvertFrom converts from the Hub version (v1beta1 -> v1alpha1) to this version.
func (lp *LogPipeline) ConvertFrom(srcRaw conversion.Hub) error {
	dst := lp

	src, ok := srcRaw.(*telemetryv1beta1.LogPipeline)
	if !ok {
		return errSrcTypeUnsupportedLogPipeline
	}

	// Copy metadata
	dst.ObjectMeta = src.ObjectMeta

	// Copy Spec fields
	dst.Spec.Input.Application = convertRuntimeToAlpha(src.Spec.Input.Runtime)

	if src.Spec.Input.OTLP != nil {
		dst.Spec.Input.OTLP = &OTLPInput{
			Disabled:   src.Spec.Input.OTLP.Enabled != nil && !*src.Spec.Input.OTLP.Enabled,
			Namespaces: convertNamespaceSelectorToAlpha(src.Spec.Input.OTLP.Namespaces),
		}
	}

	for _, f := range src.Spec.Files {
		dst.Spec.Files = append(dst.Spec.Files, LogPipelineFileMount(f))
	}

	for _, f := range src.Spec.FluentBitFilters {
		dst.Spec.FluentBitFilters = append(dst.Spec.FluentBitFilters, LogPipelineFilter(f))
	}

	if srcHTTPOutput := src.Spec.Output.HTTP; srcHTTPOutput != nil {
		dst.Spec.Output.HTTP = &LogPipelineHTTPOutput{
			Host:     convertValueTypeToAlpha(srcHTTPOutput.Host),
			URI:      srcHTTPOutput.URI,
			Port:     srcHTTPOutput.Port,
			Compress: srcHTTPOutput.Compress,
			Format:   srcHTTPOutput.Format,
			TLS:      convertOutputTLSToAlpha(srcHTTPOutput.TLSConfig),
			Dedot:    srcHTTPOutput.Dedot,
		}

		dst.Spec.Output.HTTP.User = convertValueTypeToAlphaPtr(srcHTTPOutput.User)
		dst.Spec.Output.HTTP.Password = convertValueTypeToAlphaPtr(srcHTTPOutput.Password)
	}

	if src.Spec.Output.OTLP != nil {
		dst.Spec.Output.OTLP = convertOTLPOutputToAlpha(src.Spec.Output.OTLP)
	}

	if srcCustomOutput := src.Spec.Output.Custom; srcCustomOutput != "" {
		dst.Spec.Output.Custom = srcCustomOutput
	}

	if src.Spec.Transforms != nil {
		for _, t := range src.Spec.Transforms {
			dst.Spec.Transforms = append(dst.Spec.Transforms, convertTransformSpecToAlpha(t))
		}
	}

	if src.Spec.Filters != nil {
		for _, t := range src.Spec.Filters {
			dst.Spec.Filters = append(dst.Spec.Filters, convertFilterSpecToAlpha(t))
		}
	}

	dst.Status = LogPipelineStatus(src.Status)

	return nil
}

func convertRuntimeToAlpha(runtime *telemetryv1beta1.LogPipelineRuntimeInput) *LogPipelineApplicationInput {
	if runtime == nil {
		return nil
	}

	application := &LogPipelineApplicationInput{
		Enabled: runtime.Enabled,
		Namespaces: LogPipelineNamespaceSelector{
			Include: runtime.Namespaces.Include,
			Exclude: runtime.Namespaces.Exclude,
			System:  false,
		},
		Containers: LogPipelineContainerSelector{
			Include: runtime.Containers.Include,
			Exclude: runtime.Containers.Exclude,
		},
		KeepAnnotations:  runtime.KeepAnnotations,
		DropLabels:       runtime.DropLabels,
		KeepOriginalBody: runtime.KeepOriginalBody,
	}

	return application
}

func convertOutputTLSToAlpha(src telemetryv1beta1.OutputTLS) LogPipelineOutputTLS {
	var dst LogPipelineOutputTLS

	dst.CA = convertValueTypeToAlphaPtr(src.CA)
	dst.Cert = convertValueTypeToAlphaPtr(src.Cert)
	dst.Key = convertValueTypeToAlphaPtr(src.Key)

	dst.Disabled = src.Disabled
	dst.SkipCertificateValidation = src.SkipCertificateValidation

	return dst
}
