package pipelines

import (
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func IsValid(v *telemetryv1alpha1.ValueType) bool {
	if v == nil {
		return false
	}

	if v.Value != "" {
		return true
	}

	return v.ValueFrom != nil &&
		v.ValueFrom.SecretKeyRef != nil &&
		v.ValueFrom.SecretKeyRef.Name != "" &&
		v.ValueFrom.SecretKeyRef.Key != "" &&
		v.ValueFrom.SecretKeyRef.Namespace != ""
}

func NamespacedName(skr *telemetryv1alpha1.SecretKeyRef) types.NamespacedName {
	return types.NamespacedName{Name: skr.Name, Namespace: skr.Namespace}
}
