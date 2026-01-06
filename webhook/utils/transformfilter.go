package utils

import (
	"context"
	"errors"

	apiconversion "k8s.io/apimachinery/pkg/conversion"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
)

var errFailedToCreatePipeline = errors.New("failed to create pipeline")

func ValidateFilterTransform(ctx context.Context, signalType ottl.SignalType, filterSpec []telemetryv1beta1.FilterSpec, transformSpec []telemetryv1beta1.TransformSpec) error {
	filterValidator, err := ottl.NewFilterSpecValidator(signalType)
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to instantiate FilterSpec validator")
		return errFailedToCreatePipeline
	}

	for _, filter := range filterSpec {
		err := filterValidator.ValidateConditions(filter.Conditions)
		if err != nil {
			return err
		}
	}

	transformValidator, err := ottl.NewTransformSpecValidator(signalType)
	if err != nil {
		logf.Log.V(1).Error(err, "Failed to instantiate TransformSpec validator")
		return errFailedToCreatePipeline
	}

	for _, transform := range transformSpec {
		err := transformValidator.ValidateStatementsAndConditions(transform.Statements, transform.Conditions)
		if err != nil {
			return err
		}
	}

	return nil
}

func ConvertFilterTransformToBeta(filters []telemetryv1alpha1.FilterSpec, transforms []telemetryv1alpha1.TransformSpec) ([]telemetryv1beta1.FilterSpec, []telemetryv1beta1.TransformSpec, error) {
	filterSpecs, err := convertSlice(filters, telemetryv1alpha1.Convert_v1alpha1_FilterSpec_To_v1beta1_FilterSpec)
	if err != nil {
		return nil, nil, err
	}

	transformSpecs, err := convertSlice(transforms, telemetryv1alpha1.Convert_v1alpha1_TransformSpec_To_v1beta1_TransformSpec)
	if err != nil {
		return nil, nil, err
	}

	return filterSpecs, transformSpecs, nil
}

func convertSlice[E1, E2 any](s []E1, convert func(*E1, *E2, apiconversion.Scope) error) ([]E2, error) {
	results := make([]E2, len(s))
	for i := range s {
		if err := convert(&s[i], &results[i], nil); err != nil {
			return nil, err
		}
	}

	return results, nil
}
