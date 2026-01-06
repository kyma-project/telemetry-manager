package v1alpha1

import (
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	"k8s.io/utils/ptr"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/namespaces"
)

// Converts shared structs between v1alpha1 and v1beta1 CRDs.
// Major API changes which require specific conversion logic are:
// - input.otlp.Disabled (v1alpha1) is renamed to input.otlp.Enabled (v1beta1) and its logic is inverted.
// - output.otlp.protocol is now of type enum in v1beta1 instead of string in v1alpha1.
// - output.otlp.TLS struct got renamed

// Remove invalid namespace names from NamespaceSelector slices (include/exclude)
func sanitizeNamespaceNames(names []string) []string {
	var valid []string
	// Kubernetes namespace regex
	for _, n := range names {
		if len(n) <= 63 && namespaces.ValidNameRegexp.MatchString(n) {
			valid = append(valid, n)
		}
	}

	return valid
}

func Convert_v1alpha1_OTLPInput_To_v1beta1_OTLPInput(in *OTLPInput, out *telemetryv1beta1.OTLPInput, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha1_OTLPInput_To_v1beta1_OTLPInput(in, out, s); err != nil {
		return err
	}

	out.Enabled = ptr.To(!in.Disabled)

	if in.Namespaces != nil {
		out.Namespaces = &telemetryv1beta1.NamespaceSelector{
			Include: sanitizeNamespaceNames(in.Namespaces.Include),
			Exclude: sanitizeNamespaceNames(in.Namespaces.Exclude),
		}
	}

	return nil
}

func Convert_v1beta1_OTLPInput_To_v1alpha1_OTLPInput(in *telemetryv1beta1.OTLPInput, out *OTLPInput, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_OTLPInput_To_v1alpha1_OTLPInput(in, out, s); err != nil {
		return err
	}

	out.Disabled = in.Enabled != nil && !ptr.Deref(in.Enabled, false)
	if in.Namespaces != nil {
		out.Namespaces = &NamespaceSelector{
			Include: sanitizeNamespaceNames(in.Namespaces.Include),
			Exclude: sanitizeNamespaceNames(in.Namespaces.Exclude),
		}
	}

	return nil
}
