package telemetry

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
)

const DefaultTelemetryInstanceName = "default"
const TelemetryCompatibilityModeAnnotationName = "telemetry.kyma-project.io/internal-metrics-compatibility-mode"

func GetDefaultTelemetryInstance(ctx context.Context, client client.Client, namespace string) (operatorv1alpha1.Telemetry, error) {
	var telemetry operatorv1alpha1.Telemetry

	telemetryName := types.NamespacedName{
		Namespace: namespace,
		Name:      DefaultTelemetryInstanceName,
	}

	if err := client.Get(ctx, telemetryName, &telemetry); err != nil {
		return telemetry, err
	}

	return telemetry, nil
}

func GetCompatibilityModeFromTelemetry(ctx context.Context, client client.Client, namespace string) bool {
	telemetry, err := GetDefaultTelemetryInstance(ctx, client, namespace)
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to get telemetry: using default compatibility mode")
		return false
	}

	if value, exists := telemetry.Annotations[TelemetryCompatibilityModeAnnotationName]; exists {
		return value == "true"
	}

	return false
}
