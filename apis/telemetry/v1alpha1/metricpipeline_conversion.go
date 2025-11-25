package v1alpha1

import (
	"errors"

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

// Converts between v1alpha1 and v1beta1 MetricPipeline CRDs
// There are no major changes to the MetricPipeline API between v1alpha1 and v1beta1.
// However, some changes were done in shared types which are documented in the related file and require to convert MetricPipelines.

var errSrcTypeUnsupportedMetricPipeline = errors.New("source type is not MetricPipeline v1alpha1")
var errDstTypeUnsupportedMetricPipeline = errors.New("destination type is not MetricPipeline v1beta1")

// ConvertTo implements conversion.Hub for MetricPipeline (v1alpha1 -> v1beta1)
func (src *MetricPipeline) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*telemetryv1beta1.MetricPipeline)
	if !ok {
		return errDstTypeUnsupportedMetricPipeline
	}

	// Copy metadata
	dst.ObjectMeta = src.ObjectMeta

	// Copy input fields
	dst.Spec.Input = telemetryv1beta1.MetricPipelineInput{}
	if src.Spec.Input.Prometheus != nil {
		dst.Spec.Input.Prometheus = &telemetryv1beta1.MetricPipelinePrometheusInput{
			Enabled:           src.Spec.Input.Prometheus.Enabled,
			Namespaces:        convertNamespaceSelectorToBeta(src.Spec.Input.Prometheus.Namespaces),
			DiagnosticMetrics: convertDiagnosticMetricsToBeta(src.Spec.Input.Prometheus.DiagnosticMetrics),
		}
	}

	if src.Spec.Input.Runtime != nil {
		dst.Spec.Input.Runtime = &telemetryv1beta1.MetricPipelineRuntimeInput{
			Enabled:    src.Spec.Input.Runtime.Enabled,
			Namespaces: convertNamespaceSelectorToBeta(src.Spec.Input.Runtime.Namespaces),
			Resources:  convertRuntimeResourcesToBeta(src.Spec.Input.Runtime.Resources),
		}
	}

	if src.Spec.Input.Istio != nil {
		dst.Spec.Input.Istio = &telemetryv1beta1.MetricPipelineIstioInput{
			Enabled:           src.Spec.Input.Istio.Enabled,
			Namespaces:        convertNamespaceSelectorToBeta(src.Spec.Input.Istio.Namespaces),
			DiagnosticMetrics: convertDiagnosticMetricsToBeta(src.Spec.Input.Istio.DiagnosticMetrics),
			EnvoyMetrics:      convertEnvoyMetricsToBeta(src.Spec.Input.Istio.EnvoyMetrics),
		}
	}

	dst.Spec.Input.OTLP = convertOTLPInputToBeta(src.Spec.Input.OTLP)

	// Copy output fields
	dst.Spec.Output = telemetryv1beta1.MetricPipelineOutput{}
	dst.Spec.Output.OTLP = convertOTLPOutputToBeta(src.Spec.Output.OTLP)

	// Copy everything else
	if src.Spec.Transforms != nil {
		for _, t := range src.Spec.Transforms {
			dst.Spec.Transforms = append(dst.Spec.Transforms, ConvertTransformSpecToBeta(t))
		}
	}

	if src.Spec.Filters != nil {
		for _, t := range src.Spec.Filters {
			dst.Spec.Filters = append(dst.Spec.Filters, ConvertFilterSpecToBeta(t))
		}
	}

	dst.Status = telemetryv1beta1.MetricPipelineStatus(src.Status)

	return nil
}

// ConvertFrom implements conversion.Hub for MetricPipeline (v1beta1 -> v1alpha1)
func (dst *MetricPipeline) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*telemetryv1beta1.MetricPipeline)
	if !ok {
		return errSrcTypeUnsupportedMetricPipeline
	}

	// Copy metadata
	dst.ObjectMeta = src.ObjectMeta

	// Copy input fields
	dst.Spec.Input = MetricPipelineInput{}
	if src.Spec.Input.Prometheus != nil {
		dst.Spec.Input.Prometheus = &MetricPipelinePrometheusInput{
			Enabled:           src.Spec.Input.Prometheus.Enabled,
			Namespaces:        convertNamespaceSelectorToAlpha(src.Spec.Input.Prometheus.Namespaces),
			DiagnosticMetrics: convertDiagnosticMetricsToAlpha(src.Spec.Input.Prometheus.DiagnosticMetrics),
		}
	}

	if src.Spec.Input.Runtime != nil {
		dst.Spec.Input.Runtime = &MetricPipelineRuntimeInput{
			Enabled:    src.Spec.Input.Runtime.Enabled,
			Namespaces: convertNamespaceSelectorToAlpha(src.Spec.Input.Runtime.Namespaces),
			Resources:  convertRuntimeResourcesToAlpha(src.Spec.Input.Runtime.Resources),
		}
	}

	if src.Spec.Input.Istio != nil {
		dst.Spec.Input.Istio = &MetricPipelineIstioInput{
			Enabled:           src.Spec.Input.Istio.Enabled,
			Namespaces:        convertNamespaceSelectorToAlpha(src.Spec.Input.Istio.Namespaces),
			DiagnosticMetrics: convertDiagnosticMetricsToAlpha(src.Spec.Input.Istio.DiagnosticMetrics),
			EnvoyMetrics:      convertEnvoyMetricsToAlpha(src.Spec.Input.Istio.EnvoyMetrics),
		}
	}

	dst.Spec.Input.OTLP = convertOTLPInputToAlpha(src.Spec.Input.OTLP)

	// Copy output fields
	dst.Spec.Output = MetricPipelineOutput{}
	if src.Spec.Output.OTLP != nil {
		dst.Spec.Output.OTLP = convertOTLPOutputToAlpha(src.Spec.Output.OTLP)
	}

	// Copy everything else
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

	dst.Status = MetricPipelineStatus(src.Status)

	return nil
}

// Helper conversion functions
func convertDiagnosticMetricsToBeta(dm *MetricPipelineIstioInputDiagnosticMetrics) *telemetryv1beta1.MetricPipelineIstioInputDiagnosticMetrics {
	if dm == nil {
		return nil
	}

	return &telemetryv1beta1.MetricPipelineIstioInputDiagnosticMetrics{
		Enabled: dm.Enabled,
	}
}

func convertDiagnosticMetricsToAlpha(dm *telemetryv1beta1.MetricPipelineIstioInputDiagnosticMetrics) *MetricPipelineIstioInputDiagnosticMetrics {
	if dm == nil {
		return nil
	}

	return &MetricPipelineIstioInputDiagnosticMetrics{
		Enabled: dm.Enabled,
	}
}

func convertEnvoyMetricsToBeta(em *EnvoyMetrics) *telemetryv1beta1.EnvoyMetrics {
	if em == nil {
		return nil
	}

	return &telemetryv1beta1.EnvoyMetrics{
		Enabled: em.Enabled,
	}
}

func convertEnvoyMetricsToAlpha(em *telemetryv1beta1.EnvoyMetrics) *EnvoyMetrics {
	if em == nil {
		return nil
	}

	return &EnvoyMetrics{
		Enabled: em.Enabled,
	}
}

func convertRuntimeResourcesToBeta(r *MetricPipelineRuntimeInputResources) *telemetryv1beta1.MetricPipelineRuntimeInputResources {
	if r == nil {
		return nil
	}

	return &telemetryv1beta1.MetricPipelineRuntimeInputResources{
		Pod:         convertRuntimeResourceToBeta(r.Pod),
		Container:   convertRuntimeResourceToBeta(r.Container),
		Node:        convertRuntimeResourceToBeta(r.Node),
		Volume:      convertRuntimeResourceToBeta(r.Volume),
		DaemonSet:   convertRuntimeResourceToBeta(r.DaemonSet),
		Deployment:  convertRuntimeResourceToBeta(r.Deployment),
		StatefulSet: convertRuntimeResourceToBeta(r.StatefulSet),
		Job:         convertRuntimeResourceToBeta(r.Job),
	}
}

func convertRuntimeResourcesToAlpha(r *telemetryv1beta1.MetricPipelineRuntimeInputResources) *MetricPipelineRuntimeInputResources {
	if r == nil {
		return nil
	}

	return &MetricPipelineRuntimeInputResources{
		Pod:         convertRuntimeResourceToAlpha(r.Pod),
		Container:   convertRuntimeResourceToAlpha(r.Container),
		Node:        convertRuntimeResourceToAlpha(r.Node),
		Volume:      convertRuntimeResourceToAlpha(r.Volume),
		DaemonSet:   convertRuntimeResourceToAlpha(r.DaemonSet),
		Deployment:  convertRuntimeResourceToAlpha(r.Deployment),
		StatefulSet: convertRuntimeResourceToAlpha(r.StatefulSet),
		Job:         convertRuntimeResourceToAlpha(r.Job),
	}
}

func convertRuntimeResourceToBeta(r *MetricPipelineRuntimeInputResource) *telemetryv1beta1.MetricPipelineRuntimeInputResource {
	if r == nil {
		return nil
	}

	return &telemetryv1beta1.MetricPipelineRuntimeInputResource{
		Enabled: r.Enabled,
	}
}

func convertRuntimeResourceToAlpha(r *telemetryv1beta1.MetricPipelineRuntimeInputResource) *MetricPipelineRuntimeInputResource {
	if r == nil {
		return nil
	}

	return &MetricPipelineRuntimeInputResource{
		Enabled: r.Enabled,
	}
}
