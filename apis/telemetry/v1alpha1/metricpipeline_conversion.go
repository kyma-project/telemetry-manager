package v1alpha1

import (
	"errors"

	apiconversion "k8s.io/apimachinery/pkg/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

// Converts between v1alpha1 and v1beta1 MetricPipeline CRDs
// There are no major changes to the MetricPipeline API between v1alpha1 and v1beta1.
// However, some changes were done in shared types which are documented in the related file and require to convert MetricPipelines.

var errSrcTypeUnsupportedMetricPipeline = errors.New("source type is not MetricPipeline v1alpha1")
var errDstTypeUnsupportedMetricPipeline = errors.New("destination type is not MetricPipeline v1beta1")

// ConvertTo implements conversion.Hub for MetricPipeline (v1alpha1 -> v1beta1)
func (mp *MetricPipeline) ConvertTo(dstRaw conversion.Hub) error {
	src := mp

	dst, ok := dstRaw.(*telemetryv1beta1.MetricPipeline)
	if !ok {
		return errDstTypeUnsupportedMetricPipeline
	}

	// Call the conversion-gen generated function
	return Convert_v1alpha1_MetricPipeline_To_v1beta1_MetricPipeline(src, dst, nil)
}

func (mp *MetricPipeline) ConvertFrom(srcRaw conversion.Hub) error {
	dst := mp

	src, ok := srcRaw.(*telemetryv1beta1.MetricPipeline)
	if !ok {
		return errSrcTypeUnsupportedMetricPipeline
	}

	// Call the conversion-gen generated function
	return Convert_v1beta1_MetricPipeline_To_v1alpha1_MetricPipeline(src, dst, nil)
}

func Convert_v1alpha1_MetricPipelineRuntimeInput_To_v1beta1_MetricPipelineRuntimeInput(in *MetricPipelineRuntimeInput, out *telemetryv1beta1.MetricPipelineRuntimeInput, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha1_MetricPipelineRuntimeInput_To_v1beta1_MetricPipelineRuntimeInput(in, out, s); err != nil {
		return err
	}

	if in.Namespaces != nil {
		out.Namespaces.Include = sanitizeNamespaceNames(in.Namespaces.Include)
		out.Namespaces.Exclude = sanitizeNamespaceNames(in.Namespaces.Exclude)
	}

	return nil
}
