package metric

import (
	"fmt"

	"github.com/google/uuid"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetry "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend/tls"
)

const version = "1.0.0"

type Pipeline struct {
	persistent bool

	id              string
	name            string
	otlpEndpointRef *telemetry.SecretKeyRef
	otlpEndpoint    string
	runtime         bool
	prometheus      bool
	tls             *telemetry.OtlpTLS
}

func NewPipeline(name string) *Pipeline {
	return &Pipeline{
		id:           uuid.New().String(),
		name:         name,
		otlpEndpoint: "http://unreachable:4317",
	}
}

func (p *Pipeline) WithOutputEndpoint(otlpEndpoint string) *Pipeline {
	p.otlpEndpoint = otlpEndpoint
	return p
}

func (p *Pipeline) WithOutputEndpointFromSecret(otlpEndpointRef *telemetry.SecretKeyRef) *Pipeline {
	p.otlpEndpointRef = otlpEndpointRef
	return p
}

func (p *Pipeline) Name() string {
	if p.persistent {
		return p.name
	}

	return fmt.Sprintf("%s-%s", p.name, p.id)
}

func (p *Pipeline) Persistent(persistent bool) *Pipeline {
	p.persistent = persistent

	return p
}

func (p *Pipeline) RuntimeInput(enableRuntime bool) *Pipeline {
	p.runtime = enableRuntime

	return p
}

func (p *Pipeline) PrometheusInput(enablePrometheus bool) *Pipeline {
	p.prometheus = enablePrometheus

	return p
}

func (p *Pipeline) WithTLS(certs tls.Certs) *Pipeline {
	p.tls = &telemetry.OtlpTLS{
		Insecure:           false,
		InsecureSkipVerify: false,
		CA: &telemetry.ValueType{
			Value: certs.CaCertPem.String(),
		},
		Cert: &telemetry.ValueType{
			Value: certs.ClientCertPem.String(),
		},
		Key: &telemetry.ValueType{
			Value: certs.ClientKeyPem.String(),
		},
	}

	return p
}

func (p *Pipeline) K8sObject() *telemetry.MetricPipeline {
	var labels k8s.Labels
	if p.persistent {
		labels = k8s.PersistentLabel
	}
	labels.Version(version)

	otlpOutput := &telemetry.OtlpOutput{
		Endpoint: telemetry.ValueType{},
		TLS:      p.tls,
	}
	if p.otlpEndpointRef != nil {
		otlpOutput.Endpoint.ValueFrom = &telemetry.ValueFromSource{
			SecretKeyRef: p.otlpEndpointRef,
		}
	} else {
		otlpOutput.Endpoint.Value = p.otlpEndpoint
	}

	metricPipeline := telemetry.MetricPipeline{
		ObjectMeta: k8smeta.ObjectMeta{
			Name:   p.Name(),
			Labels: labels,
		},
		Spec: telemetry.MetricPipelineSpec{
			Input: telemetry.MetricPipelineInput{
				Application: telemetry.MetricPipelineApplicationInput{
					Runtime: telemetry.MetricPipelineContainerRuntimeInput{
						Enabled: p.runtime,
					},
					Prometheus: telemetry.MetricPipelinePrometheusInput{
						Enabled: p.prometheus,
					},
				},
			},
			Output: telemetry.MetricPipelineOutput{
				Otlp: otlpOutput,
			},
		},
	}

	return &metricPipeline
}
