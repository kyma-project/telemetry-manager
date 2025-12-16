package v1alpha1

import (
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/utils/ptr"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

// Converts shared structs between v1alpha1 and v1beta1 CRDs.
// Major API changes which require specific conversion logic are:
// - input.otlp.Disabled (v1alpha1) is renamed to input.otlp.Enabled (v1beta1) and its logic is inverted.
// - output.otlp.protocol is now of type enum in v1beta1 instead of string in v1alpha1.
// - output.otlp.TLS struct got renamed

func Convert_v1alpha1_OTLPInput_To_v1beta1_OTLPInput(in *OTLPInput, out *telemetryv1beta1.OTLPInput, s conversion.Scope) error {
	if err := autoConvert_v1alpha1_OTLPInput_To_v1beta1_OTLPInput(in, out, s); err != nil {
		return err
	}

	out.Enabled = ptr.To(!in.Disabled)
	out.Namespaces = (*telemetryv1beta1.NamespaceSelector)(in.Namespaces)

	return nil
}

func Convert_v1beta1_OTLPInput_To_v1alpha1_OTLPInput(in *telemetryv1beta1.OTLPInput, out *OTLPInput, s conversion.Scope) error {
	if err := autoConvert_v1beta1_OTLPInput_To_v1alpha1_OTLPInput(in, out, s); err != nil {
		return err
	}

	out.Disabled = in.Enabled != nil && !ptr.Deref(in.Enabled, false)
	out.Namespaces = (*NamespaceSelector)(in.Namespaces)

	return nil
}
