package v1alpha1

import (
	"errors"

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

var errSrcTypeUnsupportedMetricPipeline = errors.New("source type is not MetricPipeline v1alpha1")
var errDstTypeUnsupportedMetricPipeline = errors.New("destination type is not MetricPipeline v1beta1")

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
