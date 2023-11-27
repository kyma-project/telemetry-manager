package testutils

import (
	"fmt"
	"k8s.io/utils/pointer"
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
	otlp              telemetryv1alpha1.MetricPipelineOtlpInput
	runtime           telemetryv1alpha1.MetricPipelineRuntimeInput
	prometheus        telemetryv1alpha1.MetricPipelinePrometheusInput
	istio             telemetryv1alpha1.MetricPipelineIstioInput
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
	}
}

func ExcludeNamespaces(namespaces ...string) InputOptions {
	return func(selector *telemetryv1alpha1.MetricPipelineInputNamespaceSelector) {
		selector.Exclude = namespaces
	}
}

func IncludeSystemNamespaces() InputOptions {
	return func(selector *telemetryv1alpha1.MetricPipelineInputNamespaceSelector) {
		selector.System = pointer.Bool(true)
	}
}

func (b *MetricPipelineBuilder) OtlpInput(enable bool, opts ...InputOptions) *MetricPipelineBuilder {
	b.otlp = telemetryv1alpha1.MetricPipelineOtlpInput{
		Enabled: pointer.Bool(enable),
	}
	for _, opt := range opts {
		opt(&b.otlp.Namespaces)
	}
	return b
}

func (b *MetricPipelineBuilder) RuntimeInput(enable bool, opts ...InputOptions) *MetricPipelineBuilder {
	b.runtime = telemetryv1alpha1.MetricPipelineRuntimeInput{
		Enabled: pointer.Bool(enable),
	}
	for _, opt := range opts {
		opt(&b.runtime.Namespaces)
	}
	return b
}

func (b *MetricPipelineBuilder) PrometheusInput(enable bool, opts ...InputOptions) *MetricPipelineBuilder {
	b.prometheus = telemetryv1alpha1.MetricPipelinePrometheusInput{
		Enabled: pointer.Bool(enable),
	}
	for _, opt := range opts {
		opt(&b.prometheus.Namespaces)
	}
	return b
}

func (b *MetricPipelineBuilder) IstioInput(enable bool, opts ...InputOptions) *MetricPipelineBuilder {
	b.istio = telemetryv1alpha1.MetricPipelineIstioInput{
		Enabled: pointer.Bool(enable),
	}
	for _, opt := range opts {
		opt(&b.istio.Namespaces)
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

func setDefaults(pipeline *telemetryv1alpha1.MetricPipeline) {
	input := pipeline.Spec.Input
	if input.Prometheus.Enabled == nil {
		pipeline.Spec.Input.Prometheus.Enabled = pointer.Bool(false)
	}
	if input.Runtime.Enabled == nil {
		pipeline.Spec.Input.Runtime.Enabled = pointer.Bool(false)
	}
	if input.Istio.Enabled == nil {
		pipeline.Spec.Input.Istio.Enabled = pointer.Bool(false)
	}
	if input.Otlp.Enabled == nil {
		pipeline.Spec.Input.Otlp.Enabled = pointer.Bool(true)
	}

	if input.Prometheus.Namespaces.System == nil {
		pipeline.Spec.Input.Prometheus.Namespaces.System = pointer.Bool(false)
	}
	if input.Runtime.Namespaces.System == nil {
		pipeline.Spec.Input.Runtime.Namespaces.System = pointer.Bool(false)
	}
	if input.Istio.Namespaces.System == nil {
		pipeline.Spec.Input.Istio.Namespaces.System = pointer.Bool(true)
	}
	if input.Otlp.Namespaces.System == nil {
		pipeline.Spec.Input.Otlp.Namespaces.System = pointer.Bool(false)
	}
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

	setDefaults(&pipeline)
	return pipeline
}
