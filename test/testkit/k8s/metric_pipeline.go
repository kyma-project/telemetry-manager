package k8s

import (
	"fmt"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend/tls"
)

const version = "1.0.0"

type MetricPipeline struct {
	persistent bool

	id              string
	name            string
	otlpEndpointRef *telemetryv1alpha1.SecretKeyRef
	otlpEndpoint    string
	runtime         *telemetryv1alpha1.MetricPipelineRuntimeInput
	prometheus      *telemetryv1alpha1.MetricPipelinePrometheusInput
	istio           *telemetryv1alpha1.MetricPipelineIstioInput
	otlp            *telemetryv1alpha1.MetricPipelineOtlpInput
	tls             *telemetryv1alpha1.OtlpTLS
	protocol        string
	endpointPath    string
}

func NewMetricPipeline(name string) *MetricPipeline {
	return &MetricPipeline{
		id:           uuid.New().String(),
		name:         name,
		otlpEndpoint: "http://unreachable:4317",
	}
}

func (p *MetricPipeline) WithOutputEndpoint(otlpEndpoint string) *MetricPipeline {
	p.otlpEndpoint = otlpEndpoint
	return p
}

func (p *MetricPipeline) WithOutputEndpointFromSecret(otlpEndpointRef *telemetryv1alpha1.SecretKeyRef) *MetricPipeline {
	p.otlpEndpointRef = otlpEndpointRef
	return p
}

func (p *MetricPipeline) Name() string {
	if p.persistent {
		return p.name
	}
	return fmt.Sprintf("%s-%s", p.name, p.id)
}

func (p *MetricPipeline) Persistent(persistent bool) *MetricPipeline {
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

func (p *MetricPipeline) OtlpInput(enable bool, opts ...InputOptions) *MetricPipeline {
	p.otlp = &telemetryv1alpha1.MetricPipelineOtlpInput{
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

func (p *MetricPipeline) RuntimeInput(enable bool, opts ...InputOptions) *MetricPipeline {
	p.runtime = &telemetryv1alpha1.MetricPipelineRuntimeInput{
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

func (p *MetricPipeline) PrometheusInput(enable bool, opts ...InputOptions) *MetricPipeline {
	p.prometheus = &telemetryv1alpha1.MetricPipelinePrometheusInput{
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

func (p *MetricPipeline) IstioInput(enable bool, opts ...InputOptions) *MetricPipeline {
	p.istio = &telemetryv1alpha1.MetricPipelineIstioInput{
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

func (p *MetricPipeline) PrometheusInputDiagnosticMetrics(enable bool) *MetricPipeline {
	p.prometheus.DiagnosticMetrics = &telemetryv1alpha1.DiagnosticMetrics{
		Enabled: enable,
	}
	return p
}

func (p *MetricPipeline) IstioInputDiagnosticMetrics(enable bool) *MetricPipeline {
	p.istio.DiagnosticMetrics = &telemetryv1alpha1.DiagnosticMetrics{
		Enabled: enable,
	}
	return p
}

func (p *MetricPipeline) WithTLS(certs tls.Certs) *MetricPipeline {
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

func (p *MetricPipeline) WithProtocol(protocol string) *MetricPipeline {
	p.protocol = protocol
	return p
}

func (p *MetricPipeline) WithEndpointPath(path string) *MetricPipeline {
	p.endpointPath = path
	return p
}

func (p *MetricPipeline) K8sObject() *telemetryv1alpha1.MetricPipeline {
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

	metricPipeline := telemetryv1alpha1.MetricPipeline{
		ObjectMeta: metav1.ObjectMeta{
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
