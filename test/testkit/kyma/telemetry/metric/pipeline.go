package metric

import (
	"fmt"

	"github.com/google/uuid"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend/tls"
)

const version = "1.0.0"

type Pipeline struct {
	persistent bool

	id              string
	name            string
	otlpEndpointRef *telemetryv1alpha1.SecretKeyRef
	otlpEndpoint    string
	otlp            telemetryv1alpha1.MetricPipelineOtlpInput
	runtime         telemetryv1alpha1.MetricPipelineRuntimeInput
	prometheus      telemetryv1alpha1.MetricPipelinePrometheusInput
	istio           telemetryv1alpha1.MetricPipelineIstioInput
	tls             *telemetryv1alpha1.OtlpTLS
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

type InputOptions func(selector *telemetryv1alpha1.MetricPipelineInputNamespaceSelector)

func IncludeNamespaces(namespaces ...string) InputOptions {
	return func(selector *telemetryv1alpha1.MetricPipelineInputNamespaceSelector) {
		selector.Include = namespaces
	}
}

func ExcludeNamespaces(namespaces ...string) InputOptions {
	return func(selector *telemetryv1alpha1.MetricPipelineInputNamespaceSelector) {
		selector.Exclude = namespaces
	}
}

func (p *Pipeline) OtlpInput(enable bool, opts ...InputOptions) *Pipeline {
	p.otlp = telemetryv1alpha1.MetricPipelineOtlpInput{
		Disabled: !enable,
	}

	if len(opts) == 0 {
		return p
	}

	p.otlp.Namespaces = &telemetryv1alpha1.MetricPipelineInputNamespaceSelector{}
	for _, opt := range opts {
		opt(p.otlp.Namespaces)
	}
	return p
}

func (p *Pipeline) RuntimeInput(enable bool, opts ...InputOptions) *Pipeline {
	p.runtime = telemetryv1alpha1.MetricPipelineRuntimeInput{
		Enabled: enable,
	}

	if len(opts) == 0 {
		return p
	}

	p.runtime.Namespaces = &telemetryv1alpha1.MetricPipelineInputNamespaceSelector{}
	for _, opt := range opts {
		opt(p.runtime.Namespaces)
	}
	return p
}

func (p *Pipeline) PrometheusInput(enable bool, opts ...InputOptions) *Pipeline {
	p.prometheus = telemetryv1alpha1.MetricPipelinePrometheusInput{
		Enabled: enable,
	}

	if len(opts) == 0 {
		return p
	}

	p.prometheus.Namespaces = &telemetryv1alpha1.MetricPipelineInputNamespaceSelector{}
	for _, opt := range opts {
		opt(p.prometheus.Namespaces)
	}
	return p
}

func (p *Pipeline) IstioInput(enable bool, opts ...InputOptions) *Pipeline {
	p.istio = telemetryv1alpha1.MetricPipelineIstioInput{
		Enabled: enable,
	}

	if len(opts) == 0 {
		return p
	}

	p.istio.Namespaces = &telemetryv1alpha1.MetricPipelineInputNamespaceSelector{}
	for _, opt := range opts {
		opt(p.istio.Namespaces)
	}
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

func (p *Pipeline) K8sObject() *telemetryv1alpha1.MetricPipeline {
	var labels k8s.Labels
	if p.persistent {
		labels = k8s.PersistentLabel
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

	metricPipeline := telemetryv1alpha1.MetricPipeline{
		ObjectMeta: k8smeta.ObjectMeta{
			Name:   p.Name(),
			Labels: labels,
		},
		Spec: telemetryv1alpha1.MetricPipelineSpec{
			Input: telemetryv1alpha1.MetricPipelineInput{
				Otlp:       p.otlp,
				Runtime:    p.runtime,
				Prometheus: p.prometheus,
				Istio:      p.istio,
			},
			Output: telemetryv1alpha1.MetricPipelineOutput{
				Otlp: otlpOutput,
			},
		},
	}

	return &metricPipeline
}
