package v1alpha1

import (
	"encoding/json"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
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

// dataAnnotation is the annotation that conversion webhook can use to retain the data in case of down-conversion from the hub
const dataAnnotation = "telemetry.kyma-project.io/conversion-data"

var errSrcTypeUnsupportedLogPipeline = errors.New("source type is not LogPipeline v1alpha1")
var errDstTypeUnsupportedLogPipeline = errors.New("destination type is not LogPipeline v1beta1")

func (lp *LogPipeline) ConvertTo(dstRaw conversion.Hub) error {
	src := lp

	dst, ok := dstRaw.(*telemetryv1beta1.LogPipeline)
	if !ok {
		return errDstTypeUnsupportedLogPipeline
	}

	// Call the conversion-gen generated function
	if err := Convert_v1alpha1_LogPipeline_To_v1beta1_LogPipeline(src, dst, nil); err != nil {
		return err
	}

	// It is necessary to store the v1alpha1 LogPipeline object as an annotation in the v1beta1 LogPipeline object, because the "system" field in the NamespaceSelector in v1alpha1 doesn't have a corresponding field in v1beta1
	return marshalData(src, dst)
}

func (lp *LogPipeline) ConvertFrom(srcRaw conversion.Hub) error {
	dst := lp

	src, ok := srcRaw.(*telemetryv1beta1.LogPipeline)
	if !ok {
		return errSrcTypeUnsupportedLogPipeline
	}

	// Call the conversion-gen generated function
	if err := Convert_v1beta1_LogPipeline_To_v1alpha1_LogPipeline(src, dst, nil); err != nil {
		return err
	}

	restoredV1alpha1LogPipeline := &LogPipeline{}

	ok, err := unmarshalData(src, restoredV1alpha1LogPipeline)
	if err != nil {
		return fmt.Errorf("failed to unmarshal data from annotation: %w", err)
	}

	// restore the old LogPipeline spec only if the dataAnnotation exists and there were no changes applied to the LogPipeline (same generation)
	if ok && restoredV1alpha1LogPipeline.Generation == src.Generation {
		dst.Spec = restoredV1alpha1LogPipeline.Spec
	}

	return nil
}

func Convert_v1alpha1_FluentBitHTTPOutputTLS_To_v1beta1_OutputTLS(in *FluentBitHTTPOutputTLS, out *telemetryv1beta1.OutputTLS, s apiconversion.Scope) error {
	out.Insecure = in.Disabled

	out.InsecureSkipVerify = in.SkipCertificateValidation

	if in.CA != nil {
		out.CA = &telemetryv1beta1.ValueType{}
		if err := autoConvert_v1alpha1_ValueType_To_v1beta1_ValueType(in.CA, out.CA, s); err != nil {
			return err
		}
	}

	if in.Cert != nil {
		out.Cert = &telemetryv1beta1.ValueType{}
		if err := autoConvert_v1alpha1_ValueType_To_v1beta1_ValueType(in.Cert, out.Cert, s); err != nil {
			return err
		}
	}

	if in.Key != nil {
		out.Key = &telemetryv1beta1.ValueType{}
		if err := autoConvert_v1alpha1_ValueType_To_v1beta1_ValueType(in.Key, out.Key, s); err != nil {
			return err
		}
	}

	return nil
}

func Convert_v1beta1_OutputTLS_To_v1alpha1_FluentBitHTTPOutputTLS(in *telemetryv1beta1.OutputTLS, out *FluentBitHTTPOutputTLS, s apiconversion.Scope) error {
	out.Disabled = in.Insecure

	out.SkipCertificateValidation = in.InsecureSkipVerify

	if in.CA != nil {
		out.CA = &ValueType{}
		if err := autoConvert_v1beta1_ValueType_To_v1alpha1_ValueType(in.CA, out.CA, s); err != nil {
			return err
		}
	}

	if in.Cert != nil {
		out.Cert = &ValueType{}
		if err := autoConvert_v1beta1_ValueType_To_v1alpha1_ValueType(in.Cert, out.Cert, s); err != nil {
			return err
		}
	}

	if in.Key != nil {
		out.Key = &ValueType{}
		if err := autoConvert_v1beta1_ValueType_To_v1alpha1_ValueType(in.Key, out.Key, s); err != nil {
			return err
		}
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
		Enabled:                  in.Application.Enabled,
		FluentBitKeepAnnotations: in.Application.FluentBitKeepAnnotations,
		FluentBitDropLabels:      in.Application.FluentBitDropLabels,
		KeepOriginalBody:         in.Application.KeepOriginalBody,
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

	if in.Application.Containers.Include != nil || in.Application.Containers.Exclude != nil {
		out.Runtime.Containers = &telemetryv1beta1.LogPipelineContainerSelector{
			Include: in.Application.Containers.Include,
			Exclude: in.Application.Containers.Exclude,
		}
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
		Enabled:                  in.Runtime.Enabled,
		FluentBitKeepAnnotations: in.Runtime.FluentBitKeepAnnotations,
		FluentBitDropLabels:      in.Runtime.FluentBitDropLabels,
		KeepOriginalBody:         in.Runtime.KeepOriginalBody,
	}

	if in.Runtime.Namespaces != nil {
		if len(in.Runtime.Namespaces.Include) == 0 && len(in.Runtime.Namespaces.Exclude) == 0 {
			out.Application.Namespaces = LogPipelineNamespaceSelector{
				System: true,
			}
		} else {
			out.Application.Namespaces = LogPipelineNamespaceSelector{
				Include: in.Runtime.Namespaces.Include,
				Exclude: in.Runtime.Namespaces.Exclude,
				System:  false,
			}
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

// marshalData stores the source object as json data in the destination object annotations map.
// It ignores the status of the source object, since there is no need to store the current status
func marshalData(src metav1.Object, dst metav1.Object) error {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(src)
	if err != nil {
		return err
	}

	delete(u, "status")

	data, err := json.Marshal(u)
	if err != nil {
		return err
	}

	annotations := dst.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	annotations[dataAnnotation] = string(data)
	dst.SetAnnotations(annotations)

	return nil
}

// unmarshalData tries to retrieve the data from the annotation and unmarshals it into the object passed as input.
func unmarshalData(from metav1.Object, to any) (bool, error) {
	annotations := from.GetAnnotations()

	data, ok := annotations[dataAnnotation]
	if !ok {
		return false, nil
	}

	if err := json.Unmarshal([]byte(data), to); err != nil {
		return false, err
	}

	delete(annotations, dataAnnotation)
	from.SetAnnotations(annotations)

	return true, nil
}
