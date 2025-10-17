package v1alpha1

import (
	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo implements conversion.Hub for MetricPipeline (v1alpha1 -> v1beta1)
func (src *MetricPipeline) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*telemetryv1beta1.MetricPipeline)
	if !ok {
		return errDstTypeUnsupported
	}

	// Copy metadata
	dst.ObjectMeta = src.ObjectMeta

	// Copy Spec fields
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
	if src.Spec.Input.OTLP != nil {
		dst.Spec.Input.OTLP = &telemetryv1beta1.OTLPInput{
			Disabled:   src.Spec.Input.OTLP.Disabled,
			Namespaces: convertNamespaceSelectorToBeta(src.Spec.Input.OTLP.Namespaces),
		}
	}
	dst.Spec.Output = telemetryv1beta1.MetricPipelineOutput{}
	if src.Spec.Output.OTLP != nil {
		dst.Spec.Output.OTLP = convertOTLPOutputToBeta(src.Spec.Output.OTLP)
	}
	for _, t := range src.Spec.Transforms {
		dst.Spec.Transforms = append(dst.Spec.Transforms, telemetryv1beta1.TransformSpec(t))
	}
	for _, f := range src.Spec.Filters {
		dst.Spec.Filter = append(dst.Spec.Filter, telemetryv1beta1.FilterSpec(f))
	}
	dst.Status = telemetryv1beta1.MetricPipelineStatus(src.Status)
	return nil
}

// ConvertFrom implements conversion.Hub for MetricPipeline (v1beta1 -> v1alpha1)
func (dst *MetricPipeline) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*v1beta1.MetricPipeline)
	if !ok {
		return nil
	}

	// Copy metadata
	dst.ObjectMeta = src.ObjectMeta

	// Copy Spec fields
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
	if src.Spec.Input.OTLP != nil {
		dst.Spec.Input.OTLP = &OTLPInput{
			Disabled:   src.Spec.Input.OTLP.Disabled,
			Namespaces: convertNamespaceSelectorToAlpha(src.Spec.Input.OTLP.Namespaces),
		}
	}
	dst.Spec.Output = MetricPipelineOutput{}
	if src.Spec.Output.OTLP != nil {
		dst.Spec.Output.OTLP = convertOTLPOutputToAlpha(src.Spec.Output.OTLP)
	}
	for _, t := range src.Spec.Transforms {
		dst.Spec.Transforms = append(dst.Spec.Transforms, TransformSpec(t))
	}
	for _, f := range src.Spec.Filter {
		dst.Spec.Filters = append(dst.Spec.Filters, FilterSpec(f))
	}
	dst.Status = MetricPipelineStatus(src.Status)
	return nil
}

// Helper conversion functions
func convertNamespaceSelectorToAlpha(ns *telemetryv1beta1.NamespaceSelector) *NamespaceSelector {
	if ns == nil {
		return nil
	}
	return &NamespaceSelector{
		Include: append([]string{}, ns.Include...),
		Exclude: append([]string{}, ns.Exclude...),
	}
}

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

func convertOTLPOutputToBeta(o *OTLPOutput) *telemetryv1beta1.OTLPOutput {
	if o == nil {
		return nil
	}
	return &telemetryv1beta1.OTLPOutput{
		Protocol:       telemetryv1beta1.OTLPProtocol(o.Protocol),
		Endpoint:       convertValueTypeToBeta(o.Endpoint),
		Path:           o.Path,
		Authentication: convertAuthenticationToBeta(o.Authentication),
		Headers:        convertHeadersToBeta(o.Headers),
		TLS:            convertOTLPTLSToBeta(o.TLS),
	}
}

func convertOTLPOutputToAlpha(o *telemetryv1beta1.OTLPOutput) *OTLPOutput {
	if o == nil {
		return nil
	}
	return &OTLPOutput{
		Protocol:       string(o.Protocol),
		Endpoint:       convertValueTypeToAlpha(o.Endpoint),
		Path:           o.Path,
		Authentication: convertAuthenticationToAlpha(o.Authentication),
		Headers:        convertHeadersToAlpha(o.Headers),
		TLS:            convertOTLPTLSToAlpha(o.TLS),
	}
}

func convertValueTypeToBeta(v ValueType) telemetryv1beta1.ValueType {
	return telemetryv1beta1.ValueType{
		Value:     v.Value,
		ValueFrom: convertValueFromSourceToBeta(v.ValueFrom),
	}
}

func convertValueTypeToAlpha(v telemetryv1beta1.ValueType) ValueType {
	return ValueType{
		Value:     v.Value,
		ValueFrom: convertValueFromSourceToAlpha(v.ValueFrom),
	}
}

func convertValueFromSourceToBeta(v *ValueFromSource) *telemetryv1beta1.ValueFromSource {
	if v == nil {
		return nil
	}
	return &telemetryv1beta1.ValueFromSource{
		SecretKeyRef: convertSecretKeyRefToBeta(v.SecretKeyRef),
	}
}

func convertValueFromSourceToAlpha(v *telemetryv1beta1.ValueFromSource) *ValueFromSource {
	if v == nil {
		return nil
	}
	return &ValueFromSource{
		SecretKeyRef: convertSecretKeyRefToAlpha(v.SecretKeyRef),
	}
}

func convertSecretKeyRefToBeta(s *SecretKeyRef) *telemetryv1beta1.SecretKeyRef {
	if s == nil {
		return nil
	}
	return &telemetryv1beta1.SecretKeyRef{
		Name:      s.Name,
		Namespace: s.Namespace,
		Key:       s.Key,
	}
}

func convertSecretKeyRefToAlpha(s *telemetryv1beta1.SecretKeyRef) *SecretKeyRef {
	if s == nil {
		return nil
	}
	return &SecretKeyRef{
		Name:      s.Name,
		Namespace: s.Namespace,
		Key:       s.Key,
	}
}

func convertAuthenticationToBeta(a *AuthenticationOptions) *telemetryv1beta1.AuthenticationOptions {
	if a == nil {
		return nil
	}
	return &telemetryv1beta1.AuthenticationOptions{
		Basic: convertBasicAuthToBeta(a.Basic),
	}
}

func convertAuthenticationToAlpha(a *telemetryv1beta1.AuthenticationOptions) *AuthenticationOptions {
	if a == nil {
		return nil
	}
	return &AuthenticationOptions{
		Basic: convertBasicAuthToAlpha(a.Basic),
	}
}

func convertBasicAuthToBeta(b *BasicAuthOptions) *telemetryv1beta1.BasicAuthOptions {
	if b == nil {
		return nil
	}
	return &telemetryv1beta1.BasicAuthOptions{
		User:     convertValueTypeToBeta(b.User),
		Password: convertValueTypeToBeta(b.Password),
	}
}

func convertBasicAuthToAlpha(b *telemetryv1beta1.BasicAuthOptions) *BasicAuthOptions {
	if b == nil {
		return nil
	}
	return &BasicAuthOptions{
		User:     convertValueTypeToAlpha(b.User),
		Password: convertValueTypeToAlpha(b.Password),
	}
}

func convertHeadersToBeta(hs []Header) []telemetryv1beta1.Header {
	var out []telemetryv1beta1.Header
	for _, h := range hs {
		out = append(out, telemetryv1beta1.Header{
			ValueType: convertValueTypeToBeta(h.ValueType),
			Name:      h.Name,
			Prefix:    h.Prefix,
		})
	}
	return out
}

func convertHeadersToAlpha(hs []telemetryv1beta1.Header) []Header {
	var out []Header
	for _, h := range hs {
		out = append(out, Header{
			ValueType: convertValueTypeToAlpha(h.ValueType),
			Name:      h.Name,
			Prefix:    h.Prefix,
		})
	}
	return out
}

func convertOTLPTLSToBeta(t *OTLPTLS) *telemetryv1beta1.OutputTLS {
	if t == nil {
		return nil
	}
	return &telemetryv1beta1.OutputTLS{
		Disabled:                  t.Insecure,
		SkipCertificateValidation: t.InsecureSkipVerify,
		CA:                        convertValueTypeToBetaPtr(t.CA),
		Cert:                      convertValueTypeToBetaPtr(t.Cert),
		Key:                       convertValueTypeToBetaPtr(t.Key),
	}
}

func convertOTLPTLSToAlpha(t *telemetryv1beta1.OutputTLS) *OTLPTLS {
	if t == nil {
		return nil
	}
	return &OTLPTLS{
		Insecure:           t.Disabled,
		InsecureSkipVerify: t.SkipCertificateValidation,
		CA:                 convertValueTypeToAlphaPtr(t.CA),
		Cert:               convertValueTypeToAlphaPtr(t.Cert),
		Key:                convertValueTypeToAlphaPtr(t.Key),
	}
}

func convertValueTypeToBetaPtr(v *ValueType) *telemetryv1beta1.ValueType {
	if v == nil {
		return nil
	}
	vt := convertValueTypeToBeta(*v)
	return &vt
}

func convertValueTypeToAlphaPtr(v *telemetryv1beta1.ValueType) *ValueType {
	if v == nil {
		return nil
	}
	vt := convertValueTypeToAlpha(*v)
	return &vt
}
