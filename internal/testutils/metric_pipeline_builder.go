package testutils

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type MetricPipelineBuilder struct {
	name              string
	namespace         string
	endpoint          string
	runtimeInputOn    bool
	prometheusInputOn bool
	basicAuthUser     string
	basicAuthPassword string
}

func NewMetricPipelineBuilder() *MetricPipelineBuilder {
	return &MetricPipelineBuilder{
		name:      fmt.Sprintf("test-%d", time.Now().Nanosecond()),
		namespace: "telemetry-system",
		endpoint:  "https://localhost",
	}
}

func (b *MetricPipelineBuilder) WithName(name string) *MetricPipelineBuilder {
	b.name = name
	return b
}

func (b *MetricPipelineBuilder) WithEndpoint(endpoint string) *MetricPipelineBuilder {
	b.endpoint = endpoint
	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputOn(on bool) *MetricPipelineBuilder {
	b.runtimeInputOn = on
	return b
}

func (b *MetricPipelineBuilder) WithPrometheusInputOn(on bool) *MetricPipelineBuilder {
	b.prometheusInputOn = on
	return b
}

func (b *MetricPipelineBuilder) WithBasicAuth(user, password string) *MetricPipelineBuilder {
	b.basicAuthUser = user
	b.basicAuthPassword = password
	return b
}

func (b *MetricPipelineBuilder) Build() telemetryv1alpha1.MetricPipeline {
	return telemetryv1alpha1.MetricPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.name,
			Namespace: b.namespace,
		},
		Spec: telemetryv1alpha1.MetricPipelineSpec{
			Input: telemetryv1alpha1.MetricPipelineInput{
				Application: telemetryv1alpha1.MetricPipelineApplicationInput{
					Runtime: telemetryv1alpha1.MetricPipelineContainerRuntimeInput{
						Enabled: b.runtimeInputOn,
					},
					Prometheus: telemetryv1alpha1.MetricPipelinePrometheusInput{
						Enabled: b.prometheusInputOn,
					},
				},
			},
			Output: telemetryv1alpha1.MetricPipelineOutput{
				Otlp: &telemetryv1alpha1.OtlpOutput{
					Endpoint: telemetryv1alpha1.ValueType{
						Value: b.endpoint,
					},
					Authentication: &telemetryv1alpha1.AuthenticationOptions{
						Basic: &telemetryv1alpha1.BasicAuthOptions{
							User: telemetryv1alpha1.ValueType{
								Value: b.basicAuthUser,
							},
							Password: telemetryv1alpha1.ValueType{
								Value: b.basicAuthPassword,
							},
						},
					},
				},
			},
		},
	}
}
