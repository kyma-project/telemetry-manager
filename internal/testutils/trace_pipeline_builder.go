package testutils

import (
	"fmt"
	"math/rand"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type TracePipelineBuilder struct {
	randSource rand.Source

	name              string
	endpoint          string
	basicAuthUser     string
	basicAuthPassword string
	statusConditions  []metav1.Condition
}

func NewTracePipelineBuilder() *TracePipelineBuilder {
	return &TracePipelineBuilder{
		randSource: rand.NewSource(time.Now().UnixNano()),
		endpoint:   "https://localhost",
	}
}

func (b *TracePipelineBuilder) WithName(name string) *TracePipelineBuilder {
	b.name = name
	return b
}

func (b *TracePipelineBuilder) WithEndpoint(endpoint string) *TracePipelineBuilder {
	b.endpoint = endpoint
	return b
}

func (b *TracePipelineBuilder) WithBasicAuth(user, password string) *TracePipelineBuilder {
	b.basicAuthUser = user
	b.basicAuthPassword = password
	return b
}

func (b *TracePipelineBuilder) WithStatusCondition(cond metav1.Condition) *TracePipelineBuilder {
	b.statusConditions = append(b.statusConditions, cond)
	return b
}

func (b *TracePipelineBuilder) Build() telemetryv1alpha1.TracePipeline {
	name := b.name
	if name == "" {
		name = fmt.Sprintf("test-%d", b.randSource.Int63())
	}

	return telemetryv1alpha1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Generation: 1,
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
		Status: telemetryv1alpha1.TracePipelineStatus{
			Conditions: b.statusConditions,
		},
	}
}
