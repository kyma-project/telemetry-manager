//nolint:dupl //There is duplication between metricPipelineV1Beta1 and metricPipelineV1Alpha1, but we need them as separate builders because they are using different API versions
package k8s

import (
	"fmt"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

type metricPipelineV1Alpha1 struct {
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

func NewMetricPipelineV1Alpha1(name string) *metricPipelineV1Alpha1 {
	return &metricPipelineV1Alpha1{
		id:           uuid.New().String(),
		name:         name,
		otlpEndpoint: "http://unreachable:4317",
	}
}

func (p *metricPipelineV1Alpha1) WithOutputEndpoint(otlpEndpoint string) *metricPipelineV1Alpha1 {
	p.otlpEndpoint = otlpEndpoint
	return p
}

func (p *metricPipelineV1Alpha1) WithOutputEndpointFromSecret(otlpEndpointRef *telemetryv1alpha1.SecretKeyRef) *metricPipelineV1Alpha1 {
	p.otlpEndpointRef = otlpEndpointRef
	return p
}

func (p *metricPipelineV1Alpha1) Name() string {
	if p.persistent {
		return p.name
	}
	return fmt.Sprintf("%s-%s", p.name, p.id)
}

func (p *metricPipelineV1Alpha1) Persistent(persistent bool) *metricPipelineV1Alpha1 {
	p.persistent = persistent
	return p
}

type InputOptionsV1Alpha1 func(selector *telemetryv1alpha1.MetricPipelineInputNamespaceSelector)

func IncludeNamespacesV1Alpha1(namespaces ...string) InputOptionsV1Alpha1 {
	return func(selector *telemetryv1alpha1.MetricPipelineInputNamespaceSelector) {
		selector.Include = namespaces
	}
}

func ExcludeNamespacesV1Alpha1(namespaces ...string) InputOptionsV1Alpha1 {
	return func(selector *telemetryv1alpha1.MetricPipelineInputNamespaceSelector) {
		selector.Exclude = namespaces
	}
}

func (p *metricPipelineV1Alpha1) OtlpInput(enable bool, opts ...InputOptionsV1Alpha1) *metricPipelineV1Alpha1 {
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

func (p *metricPipelineV1Alpha1) RuntimeInput(enable bool, opts ...InputOptionsV1Alpha1) *metricPipelineV1Alpha1 {
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

func (p *metricPipelineV1Alpha1) PrometheusInput(enable bool, opts ...InputOptionsV1Alpha1) *metricPipelineV1Alpha1 {
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

func (p *metricPipelineV1Alpha1) IstioInput(enable bool, opts ...InputOptionsV1Alpha1) *metricPipelineV1Alpha1 {
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

func (p *metricPipelineV1Alpha1) PrometheusInputDiagnosticMetrics(enable bool) *metricPipelineV1Alpha1 {
	p.prometheus.DiagnosticMetrics = &telemetryv1alpha1.DiagnosticMetrics{
		Enabled: enable,
	}
	return p
}

func (p *metricPipelineV1Alpha1) IstioInputDiagnosticMetrics(enable bool) *metricPipelineV1Alpha1 {
	p.istio.DiagnosticMetrics = &telemetryv1alpha1.DiagnosticMetrics{
		Enabled: enable,
	}
	return p
}

func (p *metricPipelineV1Alpha1) WithTLS(certs testutils.ClientCerts) *metricPipelineV1Alpha1 {
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

func (p *metricPipelineV1Alpha1) WithProtocol(protocol string) *metricPipelineV1Alpha1 {
	p.protocol = protocol
	return p
}

func (p *metricPipelineV1Alpha1) WithEndpointPath(path string) *metricPipelineV1Alpha1 {
	p.endpointPath = path
	return p
}

func (p *metricPipelineV1Alpha1) K8sObject() *telemetryv1alpha1.MetricPipeline {
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
