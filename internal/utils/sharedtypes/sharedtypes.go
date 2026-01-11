package sharedtypes

import (
	"context"
	"errors"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
)

var (
	ErrValueOrSecretRefUndefined = errors.New("either value or secret key reference must be defined")
)

func IsValid(v *telemetryv1beta1.ValueType) bool {
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

func ResolveValue(ctx context.Context, c client.Reader, value telemetryv1beta1.ValueType) ([]byte, error) {
	if value.Value != "" {
		return []byte(value.Value), nil
	}

	if value.ValueFrom.SecretKeyRef != nil {
		return secretref.GetValue(ctx, c, *value.ValueFrom.SecretKeyRef)
	}

	return nil, ErrValueOrSecretRefUndefined
}

func IsFilterDefined(filters []telemetryv1beta1.FilterSpec) bool {
	return len(filters) > 0
}

func IsTransformDefined(transforms []telemetryv1beta1.TransformSpec) bool {
	return len(transforms) > 0
}

func IsOTLPInputEnabled(input *telemetryv1beta1.OTLPInput) bool {
	return input == nil || input.Enabled == nil || *input.Enabled
}
