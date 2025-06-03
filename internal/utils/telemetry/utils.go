package telemetry

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/processors"
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

func GetEnrichmentsFromTelemetry(ctx context.Context, client client.Client, namespace string) processors.Enrichments {
	telemetry, err := GetDefaultTelemetryInstance(ctx, client, namespace)
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to get telemetry: using default enrichments configuration")
		return processors.Enrichments{}
	}

	if telemetry.Spec.Enrichments != nil {
		mapPodLabels := func(values []operatorv1alpha1.PodLabel, fn func(operatorv1alpha1.PodLabel) processors.PodLabel) []processors.PodLabel {
			var result []processors.PodLabel
			for i := range values {
				result = append(result, fn(values[i]))
			}

			return result
		}

		return processors.Enrichments{
			PodLabels: mapPodLabels(telemetry.Spec.Enrichments.ExtractPodLabels, func(value operatorv1alpha1.PodLabel) processors.PodLabel {
				return processors.PodLabel{
					Key:       value.Key,
					KeyPrefix: value.KeyPrefix,
				}
			}),
		}
	}

	return processors.Enrichments{}
}
