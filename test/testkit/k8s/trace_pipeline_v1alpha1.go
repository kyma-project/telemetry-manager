//nolint:dupl //There is duplication between tracePipelineV1Beta1 and tracePipelineV1Alpha1, but we need them as separate builders because they are using different API versions
package k8s

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/testkit/tlsgen"
)

type tracePipelineV1Alpha1 struct {
	persistent bool

	name            string
	otlpEndpointRef *telemetryv1alpha1.SecretKeyRef
	otlpEndpoint    string
	tls             *telemetryv1alpha1.OtlpTLS
	protocol        string
	endpointPath    string
}

func NewTracePipelineV1Alpha1(name string) *tracePipelineV1Alpha1 {
	return &tracePipelineV1Alpha1{
		name:         name,
		otlpEndpoint: "http://unreachable:4317",
	}
}

func (p *tracePipelineV1Alpha1) WithOutputEndpoint(otlpEndpoint string) *tracePipelineV1Alpha1 {
	p.otlpEndpoint = otlpEndpoint
	return p
}

func (p *tracePipelineV1Alpha1) WithOutputEndpointFromSecret(otlpEndpointRef *telemetryv1alpha1.SecretKeyRef) *tracePipelineV1Alpha1 {
	p.otlpEndpointRef = otlpEndpointRef
	return p
}

func (p *tracePipelineV1Alpha1) WithTLS(certs tlsgen.ClientCerts) *tracePipelineV1Alpha1 {
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

func (p *tracePipelineV1Alpha1) Name() string {
	return p.name
}

func (p *tracePipelineV1Alpha1) Persistent(persistent bool) *tracePipelineV1Alpha1 {
	p.persistent = persistent

	return p
}

func (p *tracePipelineV1Alpha1) WithProtocol(protocol string) *tracePipelineV1Alpha1 {
	p.protocol = protocol
	return p
}

func (p *tracePipelineV1Alpha1) WithEndpointPath(path string) *tracePipelineV1Alpha1 {
	p.endpointPath = path
	return p
}

func (p *tracePipelineV1Alpha1) K8sObject() *telemetryv1alpha1.TracePipeline {
	var labels Labels
	if p.persistent {
		labels = PersistentLabel
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
