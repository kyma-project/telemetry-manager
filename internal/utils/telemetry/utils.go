package telemetry

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
)

func GetDefaultTelemetryInstance(ctx context.Context, client client.Client, namespace string) (operatorv1beta1.Telemetry, error) {
	var telemetry operatorv1beta1.Telemetry

	telemetryName := types.NamespacedName{
		Namespace: namespace,
		Name:      "default",
	}

	if err := client.Get(ctx, telemetryName, &telemetry); err != nil {
		return telemetry, err
	}

	return telemetry, nil
}
