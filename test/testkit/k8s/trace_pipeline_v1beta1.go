//nolint:dupl //There is duplication between tracePipelineV1Beta1 and tracePipelineV1Alpha1, but we need them as separate builders because they are using different API versions
package k8s

import (
	"fmt"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

type tracePipelineV1Beta1 struct {
	persistent bool

	id              string
	name            string
	otlpEndpointRef *telemetryv1beta1.SecretKeyRef
	otlpEndpoint    string
	tls             *telemetryv1beta1.OTLPTLS
	protocol        telemetryv1beta1.OTLPProtocol
	endpointPath    string
}

func NewTracePipelineV1Beta1(name string) *tracePipelineV1Beta1 {
	return &tracePipelineV1Beta1{
		id:           uuid.New().String(),
		name:         name,
		otlpEndpoint: "http://unreachable:4317",
	}
}

func (p *tracePipelineV1Beta1) WithOutputEndpoint(otlpEndpoint string) *tracePipelineV1Beta1 {
	p.otlpEndpoint = otlpEndpoint
	return p
}

func (p *tracePipelineV1Beta1) WithOutputEndpointFromSecret(otlpEndpointRef *telemetryv1beta1.SecretKeyRef) *tracePipelineV1Beta1 {
	p.otlpEndpointRef = otlpEndpointRef
	return p
}

func (p *tracePipelineV1Beta1) WithTLS(certs testutils.ClientCerts) *tracePipelineV1Beta1 {
	p.tls = &telemetryv1beta1.OTLPTLS{
		Insecure:           false,
		InsecureSkipVerify: false,
		CA: &telemetryv1beta1.ValueType{
			Value: certs.CaCertPem.String(),
		},
		Cert: &telemetryv1beta1.ValueType{
			Value: certs.ClientCertPem.String(),
		},
		Key: &telemetryv1beta1.ValueType{
			Value: certs.ClientKeyPem.String(),
		},
	}

	return p
}

func (p *tracePipelineV1Beta1) Name() string {
	if p.persistent {
		return p.name
	}

	return fmt.Sprintf("%s-%s", p.name, p.id)
}

func (p *tracePipelineV1Beta1) Persistent(persistent bool) *tracePipelineV1Beta1 {
	p.persistent = persistent

	return p
}

func (p *tracePipelineV1Beta1) WithProtocol(protocol telemetryv1beta1.OTLPProtocol) *tracePipelineV1Beta1 {
	p.protocol = protocol
	return p
}

func (p *tracePipelineV1Beta1) WithEndpointPath(path string) *tracePipelineV1Beta1 {
	p.endpointPath = path
	return p
}

func (p *tracePipelineV1Beta1) K8sObject() *telemetryv1beta1.TracePipeline {
	var labels Labels
	if p.persistent {
		labels = PersistentLabel
	}
	labels.Version(version)

	otlpOutput := &telemetryv1beta1.OTLPOutput{
		Endpoint: telemetryv1beta1.ValueType{},
		TLS:      p.tls,
	}
	if p.otlpEndpointRef != nil {
		otlpOutput.Endpoint.ValueFrom = &telemetryv1beta1.ValueFromSource{
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

	pipeline := telemetryv1beta1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:   p.Name(),
			Labels: labels,
		},
		Spec: telemetryv1beta1.TracePipelineSpec{
			Output: telemetryv1beta1.TracePipelineOutput{
				OTLP: otlpOutput,
			},
		},
	}

	return &pipeline
}
