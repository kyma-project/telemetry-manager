package lokilogpipeline

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MakeLokiLogPipeline() *telemetryv1alpha1.LogPipeline {
	return &telemetryv1alpha1.LogPipeline{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "telemetry.kyma-project.io/v1alpha1",
			Kind:       "LogPipeline",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "loki",
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.Input{
				Application: telemetryv1alpha1.ApplicationInput{
					Namespaces: telemetryv1alpha1.InputNamespaces{
						System: true,
					},
				},
			},
			Output: telemetryv1alpha1.Output{
				Loki: &telemetryv1alpha1.LokiOutput{
					URL: telemetryv1alpha1.ValueType{
						Value: "http://logging-loki:3100/loki/api/v1/push",
					},
					Labels: map[string]string{
						"job": "telemetry-fluent-bit",
					},
					RemoveKeys: []string{"kubernetes", "stream"},
				},
			},
		},
	}
}
