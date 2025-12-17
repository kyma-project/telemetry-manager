package sharedtypes

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
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

func IsValidBeta(v *telemetryv1beta1.ValueType) bool {
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

func IsFilterDefined(filters []telemetryv1alpha1.FilterSpec) bool {
	return len(filters) > 0
}

func IsTransformDefined(transforms []telemetryv1alpha1.TransformSpec) bool {
	return len(transforms) > 0
}

func IsOTLPInputEnabled(input *telemetryv1alpha1.OTLPInput) bool {
	return input == nil || !input.Disabled
}
