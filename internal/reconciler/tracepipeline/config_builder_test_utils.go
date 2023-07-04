package tracepipeline

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type tracePipelineBuilder struct {
	name              string
	namespace         string
	endpoint          string
	basicAuthUser     string
	basicAuthPassword string
}

func newPipelineBuilder() *tracePipelineBuilder {
	return &tracePipelineBuilder{
		name:      fmt.Sprintf("test-%d", time.Now().Nanosecond()),
		namespace: "telemetry-system",
		endpoint:  "https://localhost",
	}
}

func (b *tracePipelineBuilder) withName(name string) *tracePipelineBuilder {
	b.name = name
	return b
}

func (b *tracePipelineBuilder) withEndpoint(endpoint string) *tracePipelineBuilder {
	b.endpoint = endpoint
	return b
}

func (b *tracePipelineBuilder) withBasicAuth(user, password string) *tracePipelineBuilder {
	b.basicAuthUser = user
	b.basicAuthPassword = password
	return b
}

func (b *tracePipelineBuilder) build() telemetryv1alpha1.TracePipeline {
	return telemetryv1alpha1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.name,
			Namespace: b.namespace,
		},
		Spec: telemetryv1alpha1.TracePipelineSpec{
			Output: telemetryv1alpha1.TracePipelineOutput{
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
