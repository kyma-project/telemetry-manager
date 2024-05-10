//nolint:dupl //There is duplication between tracePipelineV1Beta1 and tracePipelineV1Alpha1, but we need them as separate builders because they are using different API versions
package k8s

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

type tracePipelineV1Beta1 struct {
	persistent bool

	name                 string
	otlpEndpointRef      *telemetryv1beta1.SecretKeyRef
	otlpEndpoint         string
	tls                  *telemetryv1beta1.OTLPTLS
	protocol             telemetryv1beta1.OTLPProtocol
	endpointPath         string
	basicAuthUserRef     *telemetryv1beta1.SecretKeyRef
	basicAuthPasswordRef *telemetryv1beta1.SecretKeyRef
	headers              []telemetryv1beta1.Header
}

func NewTracePipelineV1Beta1(name string) *tracePipelineV1Beta1 {
	return &tracePipelineV1Beta1{
		name:         name,
		otlpEndpoint: "http://unreachable:4317",
		headers:      []telemetryv1beta1.Header{},
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
	return p.name
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

func (p *tracePipelineV1Beta1) WithBasicAuthUserFromSecret(basicAuthUserRef *telemetryv1beta1.SecretKeyRef) *tracePipelineV1Beta1 {
	p.basicAuthUserRef = basicAuthUserRef
	return p
}

func (p *tracePipelineV1Beta1) WithBasicAuthPasswordFromSecret(basicAuthPasswordRef *telemetryv1beta1.SecretKeyRef) *tracePipelineV1Beta1 {
	p.basicAuthPasswordRef = basicAuthPasswordRef
	return p
}

func (p *tracePipelineV1Beta1) WithHeader(name, prefix, value string) *tracePipelineV1Beta1 {
	p.headers = append(p.headers, telemetryv1beta1.Header{
		Name:   name,
		Prefix: prefix,
		ValueType: telemetryv1beta1.ValueType{
			Value: value,
		},
	})

	return p
}

func (p *tracePipelineV1Beta1) WithHeaderFromSecret(name string, prefix string, headerValueRef *telemetryv1beta1.SecretKeyRef) *tracePipelineV1Beta1 {
	p.headers = append(p.headers, telemetryv1beta1.Header{
		Name:   name,
		Prefix: prefix,
		ValueType: telemetryv1beta1.ValueType{
			ValueFrom: &telemetryv1beta1.ValueFromSource{
				SecretKeyRef: headerValueRef,
			},
		},
	})

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
		Authentication: &telemetryv1beta1.AuthenticationOptions{
			Basic: &telemetryv1beta1.BasicAuthOptions{},
		},
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

	if p.basicAuthUserRef != nil {
		otlpOutput.Authentication.Basic.User.ValueFrom = &telemetryv1beta1.ValueFromSource{
			SecretKeyRef: p.basicAuthUserRef,
		}
	}

	if p.basicAuthPasswordRef != nil {
		otlpOutput.Authentication.Basic.Password.ValueFrom = &telemetryv1beta1.ValueFromSource{
			SecretKeyRef: p.basicAuthPasswordRef,
		}
	}

	if len(p.headers) > 0 {
		otlpOutput.Headers = p.headers
	}

	pipeline := telemetryv1beta1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:   p.name,
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
