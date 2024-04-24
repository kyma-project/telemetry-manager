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
	tlsCert           string
	tlsKey            string
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

func (b *TracePipelineBuilder) WithTLS(tlsCert, tlsKey string) *TracePipelineBuilder {
	b.tlsCert = tlsCert
	b.tlsKey = tlsKey
	return b
}

func (b *TracePipelineBuilder) WithStatusCondition(cond metav1.Condition) *TracePipelineBuilder {
	b.statusConditions = append(b.statusConditions, cond)
	return b
}

func (b *TracePipelineBuilder) basicAuthOutput() telemetryv1alpha1.TracePipelineOutput {
	return telemetryv1alpha1.TracePipelineOutput{
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
	}
}

func (b *TracePipelineBuilder) tlsOutput() telemetryv1alpha1.TracePipelineOutput {
	return telemetryv1alpha1.TracePipelineOutput{
		Otlp: &telemetryv1alpha1.OtlpOutput{
			TLS: &telemetryv1alpha1.OtlpTLS{
				Cert: &telemetryv1alpha1.ValueType{Value: b.tlsCert},
				Key:  &telemetryv1alpha1.ValueType{Value: b.tlsKey},
			},
		},
	}
}

func (b *TracePipelineBuilder) Build() telemetryv1alpha1.TracePipeline {
	name := b.name
	if name == "" {
		name = fmt.Sprintf("test-%d", b.randSource.Int63())
	}

	tracePipeline := telemetryv1alpha1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Generation: 1,
		},
		Status: telemetryv1alpha1.TracePipelineStatus{
			Conditions: b.statusConditions,
		},
	}

	if !b.isTLSEnabled() {
		tracePipeline.Spec.Output = b.basicAuthOutput()
	} else {
		tracePipeline.Spec.Output = b.tlsOutput()
	}

	return tracePipeline
}

func (b *TracePipelineBuilder) isTLSEnabled() bool {
	return b.tlsCert != "" && b.tlsKey != ""
}
