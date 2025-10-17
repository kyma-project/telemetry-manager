package v1alpha1

import (
	"regexp"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

// Remove invalid namespace names from NamespaceSelector slices (include/exclude)
func sanitizeNamespaceNames(names []string) []string {
	var valid []string
	// Kubernetes namespace regex
	var nsRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	for _, n := range names {
		if len(n) <= 63 && nsRegex.MatchString(n) {
			valid = append(valid, n)
		}
	}

	return valid
}

func convertNamespaceSelectorToBeta(ns *NamespaceSelector) *telemetryv1beta1.NamespaceSelector {
	if ns == nil {
		return nil
	}

	return &telemetryv1beta1.NamespaceSelector{
		Include: sanitizeNamespaceNames(ns.Include),
		Exclude: sanitizeNamespaceNames(ns.Exclude),
	}
}

func convertNamespaceSelectorToAlpha(ns *telemetryv1beta1.NamespaceSelector) *NamespaceSelector {
	if ns == nil {
		return nil
	}

	return &NamespaceSelector{
		Include: append([]string{}, ns.Include...),
		Exclude: append([]string{}, ns.Exclude...),
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
	result := telemetryv1beta1.ValueType{
		Value: v.Value,
	}

	if v.ValueFrom != nil && v.ValueFrom.SecretKeyRef != nil {
		result.ValueFrom = convertValueFromSourceToBeta(v.ValueFrom)
	}

	return result
}

func convertValueTypeToAlpha(v telemetryv1beta1.ValueType) ValueType {
	result := ValueType{
		Value: v.Value,
	}
	if v.ValueFrom != nil && v.ValueFrom.SecretKeyRef != nil {
		result.ValueFrom = convertValueFromSourceToAlpha(v.ValueFrom)
	}

	return result
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

func convertTransformSpecToBeta(src TransformSpec) telemetryv1beta1.TransformSpec {
	var dst telemetryv1beta1.TransformSpec

	dst.Conditions = append(dst.Conditions, src.Conditions...)

	dst.Statements = append(dst.Statements, src.Statements...)

	return dst
}

func convertTransformSpecToAlpha(src telemetryv1beta1.TransformSpec) TransformSpec {
	var dst TransformSpec

	dst.Conditions = append(dst.Conditions, src.Conditions...)

	dst.Statements = append(dst.Statements, src.Statements...)

	return dst
}

func convertFilterSpecToBeta(src FilterSpec) telemetryv1beta1.FilterSpec {
	var dst telemetryv1beta1.FilterSpec

	dst.Conditions = append(dst.Conditions, src.Conditions...)

	return dst
}

func convertFilterSpecToAlpha(src telemetryv1beta1.FilterSpec) FilterSpec {
	var dst FilterSpec

	dst.Conditions = append(dst.Conditions, src.Conditions...)

	return dst
}
