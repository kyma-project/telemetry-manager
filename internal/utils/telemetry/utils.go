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
		enrichmentLabels := telemetry.Spec.Enrichments.ExtractPodLabels
		podLabels := make([]processors.PodLabel, 0, len(enrichmentLabels))

		for _, label := range enrichmentLabels {
			podLabels = append(podLabels, processors.PodLabel{
				Key:       label.Key,
				KeyPrefix: label.KeyPrefix,
			})
		}

		return processors.Enrichments{
			PodLabels: podLabels,
		}
	}

	return processors.Enrichments{}
}
