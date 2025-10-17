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
