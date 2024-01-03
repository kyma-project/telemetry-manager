package trace

import (
	"fmt"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend/tls"
)

const version = "1.0.0"

type Pipeline struct {
	persistent bool

	id              string
	name            string
	otlpEndpointRef *telemetryv1alpha1.SecretKeyRef
	otlpEndpoint    string
	tls             *telemetryv1alpha1.OtlpTLS
	protocol        string
	endpointPath    string
	headers         []telemetryv1alpha1.Header
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

func (p *Pipeline) WithOutputEndpointFromSecret(otlpEndpointRef *telemetryv1alpha1.SecretKeyRef) *Pipeline {
	p.otlpEndpointRef = otlpEndpointRef
	return p
}

func (p *Pipeline) WithTLS(certs tls.Certs) *Pipeline {
	p.tls = &telemetryv1alpha1.OtlpTLS{
		Insecure:           false,
		InsecureSkipVerify: false,
		CA: &telemetryv1alpha1.ValueType{
			Value: certs.CaCertPem.String(),
		},
		Cert: &telemetryv1alpha1.ValueType{
			Value: certs.ClientCertPem.String(),
		},
		Key: &telemetryv1alpha1.ValueType{
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

func (p *Pipeline) WithProtocol(protocol string) *Pipeline {
	p.protocol = protocol
	return p
}

func (p *Pipeline) WithEndpointPath(path string) *Pipeline {
	p.endpointPath = path
	return p
}

func (p *Pipeline) WithHeaders(headers []telemetryv1alpha1.Header) *Pipeline {
	p.headers = headers
	return p
}

func (p *Pipeline) K8sObject() *telemetryv1alpha1.TracePipeline {
	var labels kitk8s.Labels
	if p.persistent {
		labels = kitk8s.PersistentLabel
	}
	labels.Version(version)

	otlpOutput := &telemetryv1alpha1.OtlpOutput{
		Endpoint: telemetryv1alpha1.ValueType{},
		TLS:      p.tls,
	}
	if p.otlpEndpointRef != nil {
		otlpOutput.Endpoint.ValueFrom = &telemetryv1alpha1.ValueFromSource{
			SecretKeyRef: p.otlpEndpointRef,
		}
	} else {
		otlpOutput.Endpoint.Value = p.otlpEndpoint
	}

	if len(p.protocol) > 0 {
		otlpOutput.Protocol = p.protocol
	}

	if len(p.endpointPath) > 0 {
		otlpOutput.Path = p.endpointPath
	}

	pipeline := telemetryv1alpha1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:   p.Name(),
			Labels: labels,
		},
		Spec: telemetryv1alpha1.TracePipelineSpec{
			Output: telemetryv1alpha1.TracePipelineOutput{
				Otlp: otlpOutput,
			},
		},
	}

	return &pipeline
}
