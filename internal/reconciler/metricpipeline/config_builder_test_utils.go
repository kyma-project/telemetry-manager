package metricpipeline

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type metricPipelineBuilder struct {
	name              string
	namespace         string
	endpoint          string
	runtimeInputOn    bool
	istioInputOn      bool
	basicAuthUser     string
	basicAuthPassword string
}

func newPipelineBuilder() *metricPipelineBuilder {
	return &metricPipelineBuilder{
		name:           fmt.Sprintf("test-%d", time.Now().Nanosecond()),
		namespace:      "telemetry-system",
		endpoint:       "https://localhost",
		runtimeInputOn: false,
		istioInputOn:   false,
	}
}

func (b *metricPipelineBuilder) withName(name string) *metricPipelineBuilder {
	b.name = name
	return b
}

func (b *metricPipelineBuilder) withEndpoint(endpoint string) *metricPipelineBuilder {
	b.endpoint = endpoint
	return b
}

func (b *metricPipelineBuilder) withRuntimeInputOn(on bool) *metricPipelineBuilder {
	b.runtimeInputOn = on
	return b
}

func (b *metricPipelineBuilder) withBasicAuth(user, password string) *metricPipelineBuilder {
	b.basicAuthUser = user
	b.basicAuthPassword = password
	return b
}

func (b *metricPipelineBuilder) build() telemetryv1alpha1.MetricPipeline {
	return telemetryv1alpha1.MetricPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.name,
			Namespace: b.namespace,
		},
		Spec: telemetryv1alpha1.MetricPipelineSpec{
			Input: telemetryv1alpha1.MetricPipelineInput{
				Runtime: telemetryv1alpha1.MetricPipelineContainerRuntimeInput{
					Enabled: b.runtimeInputOn,
				},
				Istio: telemetryv1alpha1.MetricPipelineIstioInput{
					Enabled: b.istioInputOn,
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
