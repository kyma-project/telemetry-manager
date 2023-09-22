package trace

import (
	"fmt"

	"github.com/google/uuid"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetry "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
)

const version = "1.0.0"

type Pipeline struct {
	name                 string
	endpointSecretKeyRef *telemetry.SecretKeyRef
	persistent           bool
	id                   string
	tls                  *telemetry.OtlpTLS
}

func NewPipeline(name string, secretKeyRef *telemetry.SecretKeyRef) *Pipeline {
	return &Pipeline{
		name:                 name,
		endpointSecretKeyRef: secretKeyRef,
		id:                   uuid.New().String(),
	}
}

func (p *Pipeline) WithTLS(certs backend.TLSCerts) *Pipeline {
	p.tls = &telemetry.OtlpTLS{
		Insecure:           false,
		InsecureSkipVerify: false,
		CA: telemetry.ValueType{
			Value: certs.CaCertPem.String(),
		},
		Cert: telemetry.ValueType{
			Value: certs.ClientCertPem.String(),
		},
		Key: telemetry.ValueType{
			Value: certs.ClientKeyPem.String(),
		},
	}

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

func (p *Pipeline) K8sObject() *telemetry.TracePipeline {
	var labels k8s.Labels
	if p.persistent {
		labels = k8s.PersistentLabel
	}
	labels.Version(version)

	pipeline := telemetry.TracePipeline{
		ObjectMeta: k8smeta.ObjectMeta{
			Name:   p.Name(),
			Labels: labels,
		},
		Spec: telemetry.TracePipelineSpec{
			Output: telemetry.TracePipelineOutput{
				Otlp: &telemetry.OtlpOutput{
					Endpoint: telemetry.ValueType{
						ValueFrom: &telemetry.ValueFromSource{
							SecretKeyRef: p.endpointSecretKeyRef,
						},
					},
					TLS: p.tls,
				},
			},
		},
	}

	return &pipeline
}
