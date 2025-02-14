package telemetry

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
)

const DefaultTelemetryInstanceName = "default"

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
