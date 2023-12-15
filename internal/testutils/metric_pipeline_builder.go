package testutils

import (
	"fmt"
	"math/rand"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
)

type MetricPipelineBuilder struct {
	randSource rand.Source

	name              string
	endpoint          string
	runtime           *telemetryv1alpha1.MetricPipelineRuntimeInput
	prometheus        *telemetryv1alpha1.MetricPipelinePrometheusInput
	istio             *telemetryv1alpha1.MetricPipelineIstioInput
	otlp              *telemetryv1alpha1.MetricPipelineOtlpInput
	basicAuthUser     string
	basicAuthPassword string

	conditions []telemetryv1alpha1.MetricPipelineCondition
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

func MetricPendingCondition(reason string) telemetryv1alpha1.MetricPipelineCondition {
	return telemetryv1alpha1.MetricPipelineCondition{
		Reason: reason,
		Type:   telemetryv1alpha1.MetricPipelinePending,
	}
}

func MetricRunningCondition() telemetryv1alpha1.MetricPipelineCondition {
	return telemetryv1alpha1.MetricPipelineCondition{
		Reason: conditions.ReasonMetricGatewayDeploymentReady,
		Type:   telemetryv1alpha1.MetricPipelineRunning,
	}
}

func (b *MetricPipelineBuilder) WithStatusConditions(conditions ...telemetryv1alpha1.MetricPipelineCondition) *MetricPipelineBuilder {
	b.conditions = conditions
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
							},
							Password: telemetryv1alpha1.ValueType{
								Value: b.basicAuthPassword,
							},
						},
					},
				},
			},
		},
		Status: telemetryv1alpha1.MetricPipelineStatus{
			Conditions: b.conditions,
		},
	}

	return pipeline
}
