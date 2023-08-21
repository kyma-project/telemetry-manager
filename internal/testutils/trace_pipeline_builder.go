package testutils

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type TracePipelineBuilder struct {
	name              string
	namespace         string
	endpoint          string
	basicAuthUser     string
	basicAuthPassword string
	insecure          bool
	tlsCA             string
	tlsCert           string
	tlsKey            string
}

func NewTracePipelineBuilder() *TracePipelineBuilder {
	return &TracePipelineBuilder{
		name:      fmt.Sprintf("test-%d", time.Now().Nanosecond()),
		namespace: "telemetry-system",
		endpoint:  "https://localhost",
		insecure:  true,
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

func (b *TracePipelineBuilder) WithmTLS() *TracePipelineBuilder {
	b.insecure = false
	b.tlsCA = "test ca pem"
	b.tlsCert = "test cert pem"
	b.tlsKey = "test key pem"
	return b
}

func (b *TracePipelineBuilder) Build() telemetryv1alpha1.TracePipeline {
	tls := telemetryv1alpha1.OtlpTLS{
		Insecure: b.insecure,
	}
	if !b.insecure {
		tls.CA = telemetryv1alpha1.ValueType{
			Value: b.tlsCA,
		}
		tls.Cert = telemetryv1alpha1.ValueType{
			Value: b.tlsCert,
		}
		tls.Key = telemetryv1alpha1.ValueType{
			Value: b.tlsKey,
		}
	}

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
					TLS: tls,
				},
			},
		},
	}
}
