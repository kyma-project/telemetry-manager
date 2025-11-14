package utils

import (
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
)

func ValidateFilterTransform(signalType ottl.SignalType, filterSpec []telemetryv1alpha1.FilterSpec, transformSpec []telemetryv1alpha1.TransformSpec) error {
	filterValidator, err := ottl.NewFilterSpecValidator(signalType)
	if err != nil {
		return fmt.Errorf("failed to instantiate FilterSpecValidator %w", err)
	}

	err = filterValidator.Validate(filterSpec)
	if err != nil {
		return err
	}

	transformValidator, err := ottl.NewTransformSpecValidator(signalType)
	if err != nil {
		return fmt.Errorf("failed to instantiate TransformSpecValidator %w", err)
	}

	err = transformValidator.Validate(transformSpec)
	if err != nil {
		return err
	}

	return nil
}
