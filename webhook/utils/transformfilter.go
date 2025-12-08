package utils

import (
	"fmt"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	slicesutils "github.com/kyma-project/telemetry-manager/internal/utils/slices"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
)

func ValidateFilterTransform(signalType ottl.SignalType, filterSpec []telemetryv1beta1.FilterSpec, transformSpec []telemetryv1beta1.TransformSpec) error {
	filterValidator, err := ottl.NewFilterSpecValidator(signalType)
	if err != nil {
		logf.Log.V(1).Error(err, "Failed to instantiate FilterSpec validator")
		return fmt.Errorf("failed to create pipeline")
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
		return fmt.Errorf("failed to create pipeline")
	}

	for _, transform := range transformSpec {
		err := transformValidator.ValidateStatementsAndConditions(transform.Statements, transform.Conditions)
		if err != nil {
			return err
		}
	}

	return nil
}

func ConvertFilterTransformToBeta(filters []telemetryv1alpha1.FilterSpec, transforms []telemetryv1alpha1.TransformSpec) ([]telemetryv1beta1.FilterSpec, []telemetryv1beta1.TransformSpec) {
	return slicesutils.TransformFunc(filters, telemetryv1alpha1.ConvertFilterSpecToBeta), slicesutils.TransformFunc(transforms, telemetryv1alpha1.ConvertTransformSpecToBeta)
}
