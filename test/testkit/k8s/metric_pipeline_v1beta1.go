//nolint:dupl //There is duplication between metricPipelineV1Beta1 and metricPipelineV1Alpha1, but we need them as separate builders because they are using different API versions
package k8s

import (
	"fmt"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend/tls"
)

const version = "1.0.0"

type metricPipelineV1Beta1 struct {
	persistent bool

	id              string
	name            string
	otlpEndpointRef *telemetryv1beta1.SecretKeyRef
	otlpEndpoint    string
	runtime         *telemetryv1beta1.MetricPipelineRuntimeInput
	prometheus      *telemetryv1beta1.MetricPipelinePrometheusInput
	istio           *telemetryv1beta1.MetricPipelineIstioInput
	otlp            *telemetryv1beta1.MetricPipelineOTLPInput
	tls             *telemetryv1beta1.OTLPTLS
	protocol        telemetryv1beta1.OTLPProtocol
	endpointPath    string
}

func NewMetricPipelineV1Beta1(name string) *metricPipelineV1Beta1 {
	return &metricPipelineV1Beta1{
		id:           uuid.New().String(),
		name:         name,
		otlpEndpoint: "http://unreachable:4317",
	}
}

func (p *metricPipelineV1Beta1) WithOutputEndpoint(otlpEndpoint string) *metricPipelineV1Beta1 {
	p.otlpEndpoint = otlpEndpoint
	return p
}

func (p *metricPipelineV1Beta1) WithOutputEndpointFromSecret(otlpEndpointRef *telemetryv1beta1.SecretKeyRef) *metricPipelineV1Beta1 {
	p.otlpEndpointRef = otlpEndpointRef
	return p
}

func (p *metricPipelineV1Beta1) Name() string {
	if p.persistent {
		return p.name
	}
	return fmt.Sprintf("%s-%s", p.name, p.id)
}

func (p *metricPipelineV1Beta1) Persistent(persistent bool) *metricPipelineV1Beta1 {
	p.persistent = persistent
	return p
}

type InputOptionsV1Beta1 func(selector *telemetryv1beta1.MetricPipelineInputNamespaceSelector)

func IncludeNamespacesV1Beta1(namespaces ...string) InputOptionsV1Beta1 {
	return func(selector *telemetryv1beta1.MetricPipelineInputNamespaceSelector) {
		selector.Include = namespaces
	}
}

func ExcludeNamespacesV1Beta1(namespaces ...string) InputOptionsV1Beta1 {
	return func(selector *telemetryv1beta1.MetricPipelineInputNamespaceSelector) {
		selector.Exclude = namespaces
	}
}

func (p *metricPipelineV1Beta1) OtlpInput(enable bool, opts ...InputOptionsV1Beta1) *metricPipelineV1Beta1 {
	p.otlp = &telemetryv1beta1.MetricPipelineOTLPInput{
		Disabled: !enable,
	}

	if len(opts) == 0 {
		return p
	}

	p.otlp.Namespaces = &telemetryv1beta1.MetricPipelineInputNamespaceSelector{}
	for _, opt := range opts {
		opt(p.otlp.Namespaces)
	}
	return p
}

func (p *metricPipelineV1Beta1) RuntimeInput(enable bool, opts ...InputOptionsV1Beta1) *metricPipelineV1Beta1 {
	p.runtime = &telemetryv1beta1.MetricPipelineRuntimeInput{
		Enabled: enable,
	}

	if len(opts) == 0 {
		return p
	}

	p.runtime.Namespaces = &telemetryv1beta1.MetricPipelineInputNamespaceSelector{}
	for _, opt := range opts {
		opt(p.runtime.Namespaces)
	}
	return p
}

func (p *metricPipelineV1Beta1) PrometheusInput(enable bool, opts ...InputOptionsV1Beta1) *metricPipelineV1Beta1 {
	p.prometheus = &telemetryv1beta1.MetricPipelinePrometheusInput{
		Enabled: enable,
	}

	if len(opts) == 0 {
		return p
	}

	p.prometheus.Namespaces = &telemetryv1beta1.MetricPipelineInputNamespaceSelector{}
	for _, opt := range opts {
		opt(p.prometheus.Namespaces)
	}
	return p
}

func (p *metricPipelineV1Beta1) IstioInput(enable bool, opts ...InputOptionsV1Beta1) *metricPipelineV1Beta1 {
	p.istio = &telemetryv1beta1.MetricPipelineIstioInput{
		Enabled: enable,
	}

	if len(opts) == 0 {
		return p
	}

	p.istio.Namespaces = &telemetryv1beta1.MetricPipelineInputNamespaceSelector{}
	for _, opt := range opts {
		opt(p.istio.Namespaces)
	}
	return p
}

func (p *metricPipelineV1Beta1) PrometheusInputDiagnosticMetrics(enable bool) *metricPipelineV1Beta1 {
	p.prometheus.DiagnosticMetrics = &telemetryv1beta1.DiagnosticMetrics{
		Enabled: enable,
	}
	return p
}

func (p *metricPipelineV1Beta1) IstioInputDiagnosticMetrics(enable bool) *metricPipelineV1Beta1 {
	p.istio.DiagnosticMetrics = &telemetryv1beta1.DiagnosticMetrics{
		Enabled: enable,
	}
	return p
}

func (p *metricPipelineV1Beta1) WithTLS(certs tls.Certs) *metricPipelineV1Beta1 {
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

func (p *metricPipelineV1Beta1) WithProtocol(protocol telemetryv1beta1.OTLPProtocol) *metricPipelineV1Beta1 {
	p.protocol = protocol
	return p
}

func (p *metricPipelineV1Beta1) WithEndpointPath(path string) *metricPipelineV1Beta1 {
	p.endpointPath = path
	return p
}

func (p *metricPipelineV1Beta1) K8sObject() *telemetryv1beta1.MetricPipeline {
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

	metricPipeline := telemetryv1beta1.MetricPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:   p.Name(),
			Labels: labels,
		},
		Spec: telemetryv1beta1.MetricPipelineSpec{
			Input: telemetryv1beta1.MetricPipelineInput{
				OTLP:       p.otlp,
				Runtime:    p.runtime,
				Prometheus: p.prometheus,
				Istio:      p.istio,
			},
			Output: telemetryv1beta1.MetricPipelineOutput{
				OTLP: otlpOutput,
			},
		},
	}

	return &metricPipeline
}
