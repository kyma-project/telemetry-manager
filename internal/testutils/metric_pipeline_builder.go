package testutils

import (
	"fmt"
	"math/rand"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type MetricPipelineBuilder struct {
	randSource rand.Source

	name                       string
	endpoint                   string
	runtime                    *telemetryv1alpha1.MetricPipelineRuntimeInput
	prometheus                 *telemetryv1alpha1.MetricPipelinePrometheusInput
	istio                      *telemetryv1alpha1.MetricPipelineIstioInput
	otlp                       *telemetryv1alpha1.MetricPipelineOtlpInput
	basicAuthUser              string
	basicAuthPassword          string
	basicAuthSecretName        string
	basicAuthSecretNamespace   string
	basicAuthSecretUserKey     string
	basicAuthSecretPasswordKey string
	statusConditions           []metav1.Condition
}

func NewMetricPipelineBuilder() *MetricPipelineBuilder {
	return &MetricPipelineBuilder{
		randSource: rand.NewSource(time.Now().UnixNano()),
		endpoint:   "https://localhost",
	}
}

func (b *MetricPipelineBuilder) WithName(name string) *MetricPipelineBuilder {
	b.name = name
	return b
}

func (b *MetricPipelineBuilder) WithEndpoint(endpoint string) *MetricPipelineBuilder {
	b.endpoint = endpoint
	return b
}

type InputOptions func(selector *telemetryv1alpha1.MetricPipelineInputNamespaceSelector)

func IncludeNamespaces(namespaces ...string) InputOptions {
	return func(selector *telemetryv1alpha1.MetricPipelineInputNamespaceSelector) {
		selector.Include = namespaces
		selector.Exclude = nil
	}
}

func ExcludeNamespaces(namespaces ...string) InputOptions {
	return func(selector *telemetryv1alpha1.MetricPipelineInputNamespaceSelector) {
		selector.Include = nil
		selector.Exclude = namespaces
	}
}

func (b *MetricPipelineBuilder) RuntimeInput(enable bool, opts ...InputOptions) *MetricPipelineBuilder {
	if b.runtime == nil {
		b.runtime = &telemetryv1alpha1.MetricPipelineRuntimeInput{}
	}
	b.runtime.Enabled = enable

	if len(opts) == 0 {
		return b
	}

	if b.runtime.Namespaces == nil {
		b.runtime.Namespaces = &telemetryv1alpha1.MetricPipelineInputNamespaceSelector{}
	}
	for _, opt := range opts {
		opt(b.runtime.Namespaces)
	}
	return b
}

func (b *MetricPipelineBuilder) PrometheusInput(enable bool, opts ...InputOptions) *MetricPipelineBuilder {
	if b.prometheus == nil {
		b.prometheus = &telemetryv1alpha1.MetricPipelinePrometheusInput{}
	}
	b.prometheus.Enabled = enable

	if len(opts) == 0 {
		return b
	}

	if b.prometheus.Namespaces == nil {
		b.prometheus.Namespaces = &telemetryv1alpha1.MetricPipelineInputNamespaceSelector{}
	}
	for _, opt := range opts {
		opt(b.prometheus.Namespaces)
	}
	return b
}

func (b *MetricPipelineBuilder) IstioInput(enable bool, opts ...InputOptions) *MetricPipelineBuilder {
	if b.istio == nil {
		b.istio = &telemetryv1alpha1.MetricPipelineIstioInput{}
	}
	b.istio.Enabled = enable

	if len(opts) == 0 {
		return b
	}

	if b.istio.Namespaces == nil {
		b.istio.Namespaces = &telemetryv1alpha1.MetricPipelineInputNamespaceSelector{}
	}
	for _, opt := range opts {
		opt(b.istio.Namespaces)
	}
	return b
}

func (b *MetricPipelineBuilder) PrometheusInputDiagnosticMetrics(enable bool) *MetricPipelineBuilder {
	if b.prometheus == nil {
		b.prometheus = &telemetryv1alpha1.MetricPipelinePrometheusInput{}
	}

	if b.prometheus.DiagnosticMetrics == nil {
		b.prometheus.DiagnosticMetrics = &telemetryv1alpha1.DiagnosticMetrics{}
	}
	b.prometheus.DiagnosticMetrics.Enabled = enable

	return b
}

func (b *MetricPipelineBuilder) IstioInputDiagnosticMetrics(enable bool) *MetricPipelineBuilder {
	if b.istio == nil {
		b.istio = &telemetryv1alpha1.MetricPipelineIstioInput{}
	}

	if b.istio.DiagnosticMetrics == nil {
		b.istio.DiagnosticMetrics = &telemetryv1alpha1.DiagnosticMetrics{}
	}

	b.istio.DiagnosticMetrics.Enabled = enable

	return b
}

func (b *MetricPipelineBuilder) OtlpInput(enable bool, opts ...InputOptions) *MetricPipelineBuilder {
	if b.otlp == nil {
		b.otlp = &telemetryv1alpha1.MetricPipelineOtlpInput{}
	}
	b.otlp.Disabled = !enable

	if len(opts) == 0 {
		return b
	}

	if b.otlp.Namespaces == nil {
		b.otlp.Namespaces = &telemetryv1alpha1.MetricPipelineInputNamespaceSelector{}
	}
	for _, opt := range opts {
		opt(b.otlp.Namespaces)
	}
	return b
}

func (b *MetricPipelineBuilder) WithBasicAuth(user, password string) *MetricPipelineBuilder {
	b.basicAuthUser = user
	b.basicAuthPassword = password
	return b
}

func (b *MetricPipelineBuilder) WithBasicAuthFromSecret(secretName, secretNamespace, userKey, passwordKey string) *MetricPipelineBuilder {
	b.basicAuthSecretName = secretName
	b.basicAuthSecretNamespace = secretNamespace
	b.basicAuthSecretUserKey = userKey
	b.basicAuthSecretPasswordKey = passwordKey
	return b
}

func (b *MetricPipelineBuilder) WithStatusCondition(cond metav1.Condition) *MetricPipelineBuilder {
	b.statusConditions = append(b.statusConditions, cond)
	return b
}

func (b *MetricPipelineBuilder) Build() telemetryv1alpha1.MetricPipeline {
	name := b.name
	if name == "" {
		name = fmt.Sprintf("test-%d", b.randSource.Int63())
	}

	pipeline := telemetryv1alpha1.MetricPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: telemetryv1alpha1.MetricPipelineStatus{
			Conditions: b.statusConditions,
		},
		Spec: telemetryv1alpha1.MetricPipelineSpec{
			Input: telemetryv1alpha1.MetricPipelineInput{
				Runtime:    b.runtime,
				Prometheus: b.prometheus,
				Istio:      b.istio,
				Otlp:       b.otlp,
			},
			Output: telemetryv1alpha1.MetricPipelineOutput{
				Otlp: &telemetryv1alpha1.OtlpOutput{
					Endpoint: telemetryv1alpha1.ValueType{
						Value: b.endpoint,
					},
					Authentication: &telemetryv1alpha1.AuthenticationOptions{
						Basic: &telemetryv1alpha1.BasicAuthOptions{
							User: telemetryv1alpha1.ValueType{
								Value: b.basicAuthUser,
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name:      b.basicAuthSecretName,
										Namespace: b.basicAuthSecretNamespace,
										Key:       b.basicAuthSecretUserKey,
									},
								},
							},
							Password: telemetryv1alpha1.ValueType{
								Value: b.basicAuthPassword,
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name:      b.basicAuthSecretName,
										Namespace: b.basicAuthSecretNamespace,
										Key:       b.basicAuthSecretPasswordKey,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return pipeline
}
