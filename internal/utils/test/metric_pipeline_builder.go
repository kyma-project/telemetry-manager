package test

import (
	"fmt"
	"math/rand"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

type MetricPipelineBuilder struct {
	randSource rand.Source

	name        string
	labels      map[string]string
	annotations map[string]string

	inRuntime    *telemetryv1beta1.MetricPipelineRuntimeInput
	inPrometheus *telemetryv1beta1.MetricPipelinePrometheusInput
	inIstio      *telemetryv1beta1.MetricPipelineIstioInput
	inOTLP       *telemetryv1beta1.OTLPInput

	outOTLP *telemetryv1beta1.OTLPOutput
	oauth2  *telemetryv1beta1.OAuth2Options

	transforms       []telemetryv1beta1.TransformSpec
	filter           []telemetryv1beta1.FilterSpec
	statusConditions []metav1.Condition
}

func NewMetricPipelineBuilder() *MetricPipelineBuilder {
	return &MetricPipelineBuilder{
		randSource: rand.NewSource(time.Now().UnixNano()),
		outOTLP: &telemetryv1beta1.OTLPOutput{
			Endpoint: telemetryv1beta1.ValueType{Value: "http://localhost:4317"},
		},
	}
}

func buildMetricPipelineInput(
	enablePrometheus bool, prometheusOpts []NamespaceSelectorOptions,
	enableRuntime bool, runtimeOpts []NamespaceSelectorOptions,
	enableIstio bool, istioOpts []NamespaceSelectorOptions,
	enableOTLP bool, otlpOpts []NamespaceSelectorOptions,
) telemetryv1beta1.MetricPipelineInput {
	input := telemetryv1beta1.MetricPipelineInput{}

	if enablePrometheus {
		input.Prometheus = &telemetryv1beta1.MetricPipelinePrometheusInput{
			Enabled:    ptr.To(true),
			Namespaces: &telemetryv1beta1.NamespaceSelector{},
		}
		for _, opt := range prometheusOpts {
			opt(input.Prometheus.Namespaces)
		}
	} else {
		input.Prometheus = &telemetryv1beta1.MetricPipelinePrometheusInput{
			Enabled: ptr.To(false),
		}
	}

	if enableRuntime {
		input.Runtime = &telemetryv1beta1.MetricPipelineRuntimeInput{
			Enabled:    ptr.To(true),
			Namespaces: &telemetryv1beta1.NamespaceSelector{},
		}
		for _, opt := range runtimeOpts {
			opt(input.Runtime.Namespaces)
		}
	} else {
		input.Runtime = &telemetryv1beta1.MetricPipelineRuntimeInput{
			Enabled: ptr.To(false),
		}
	}

	if enableIstio {
		input.Istio = &telemetryv1beta1.MetricPipelineIstioInput{
			Enabled:    ptr.To(true),
			Namespaces: &telemetryv1beta1.NamespaceSelector{},
		}
		for _, opt := range istioOpts {
			opt(input.Istio.Namespaces)
		}
	} else {
		input.Istio = &telemetryv1beta1.MetricPipelineIstioInput{
			Enabled: ptr.To(false),
		}
	}

	if enableOTLP {
		input.OTLP = &telemetryv1beta1.OTLPInput{
			Enabled:    ptr.To(true),
			Namespaces: &telemetryv1beta1.NamespaceSelector{},
		}
		for _, opt := range otlpOpts {
			opt(input.OTLP.Namespaces)
		}
	} else {
		input.OTLP = &telemetryv1beta1.OTLPInput{
			Enabled: ptr.To(false),
		}
	}

	return input
}

func BuildMetricPipelineAgentInput(runtime, prometheus, istio bool, opts ...NamespaceSelectorOptions) telemetryv1beta1.MetricPipelineInput {
	return buildMetricPipelineInput(
		prometheus, opts,
		runtime, opts,
		istio, opts,
		false, nil,
	)
}

func BuildMetricPipelineRuntimeInput(opts ...NamespaceSelectorOptions) telemetryv1beta1.MetricPipelineInput {
	return buildMetricPipelineInput(
		false, nil,
		true, opts,
		false, nil,
		false, nil,
	)
}

func BuildMetricPipelineOTLPInput(opts ...NamespaceSelectorOptions) telemetryv1beta1.MetricPipelineInput {
	return buildMetricPipelineInput(
		false, nil,
		false, nil,
		false, nil,
		true, opts,
	)
}

func (b *MetricPipelineBuilder) WithName(name string) *MetricPipelineBuilder {
	b.name = name
	return b
}

func (b *MetricPipelineBuilder) WithLabels(labels map[string]string) *MetricPipelineBuilder {
	b.labels = labels
	return b
}

func (b *MetricPipelineBuilder) WithAnnotations(annotations map[string]string) *MetricPipelineBuilder {
	b.annotations = annotations
	return b
}

func (b *MetricPipelineBuilder) WithInput(input telemetryv1beta1.MetricPipelineInput) *MetricPipelineBuilder {
	if input.Runtime != nil {
		b.inRuntime = input.Runtime
	}

	if input.Prometheus != nil {
		b.inPrometheus = input.Prometheus
	}

	if input.Istio != nil {
		b.inIstio = input.Istio
	}

	if input.OTLP != nil {
		b.inOTLP = input.OTLP
	}

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInput(enable bool, opts ...NamespaceSelectorOptions) *MetricPipelineBuilder {
	if b.inRuntime == nil {
		b.inRuntime = &telemetryv1beta1.MetricPipelineRuntimeInput{}
	}

	b.inRuntime.Enabled = &enable

	if len(opts) == 0 {
		return b
	}

	if b.inRuntime.Namespaces == nil {
		b.inRuntime.Namespaces = &telemetryv1beta1.NamespaceSelector{}
	}

	for _, opt := range opts {
		opt(b.inRuntime.Namespaces)
	}

	return b
}

func (b *MetricPipelineBuilder) WithPrometheusInput(enable bool, opts ...NamespaceSelectorOptions) *MetricPipelineBuilder {
	if b.inPrometheus == nil {
		b.inPrometheus = &telemetryv1beta1.MetricPipelinePrometheusInput{}
	}

	b.inPrometheus.Enabled = &enable

	if len(opts) == 0 {
		return b
	}

	if b.inPrometheus.Namespaces == nil {
		b.inPrometheus.Namespaces = &telemetryv1beta1.NamespaceSelector{}
	}

	for _, opt := range opts {
		opt(b.inPrometheus.Namespaces)
	}

	return b
}

func (b *MetricPipelineBuilder) WithIstioInput(enable bool, opts ...NamespaceSelectorOptions) *MetricPipelineBuilder {
	if b.inIstio == nil {
		b.inIstio = &telemetryv1beta1.MetricPipelineIstioInput{}
	}

	b.inIstio.Enabled = &enable

	if len(opts) == 0 {
		return b
	}

	if b.inIstio.Namespaces == nil {
		b.inIstio.Namespaces = &telemetryv1beta1.NamespaceSelector{}
	}

	for _, opt := range opts {
		opt(b.inIstio.Namespaces)
	}

	return b
}

func (b *MetricPipelineBuilder) WithOTLPInput(enable bool, opts ...NamespaceSelectorOptions) *MetricPipelineBuilder {
	if b.inOTLP == nil {
		b.inOTLP = &telemetryv1beta1.OTLPInput{}
	}

	b.inOTLP.Enabled = ptr.To(enable)

	if len(opts) == 0 {
		return b
	}

	if b.inOTLP.Namespaces == nil {
		b.inOTLP.Namespaces = &telemetryv1beta1.NamespaceSelector{}
	}

	for _, opt := range opts {
		opt(b.inOTLP.Namespaces)
	}

	return b
}

func (b *MetricPipelineBuilder) WithPrometheusInputDiagnosticMetrics(enable bool) *MetricPipelineBuilder {
	if b.inPrometheus == nil {
		b.inPrometheus = &telemetryv1beta1.MetricPipelinePrometheusInput{}
	}

	if b.inPrometheus.DiagnosticMetrics == nil {
		b.inPrometheus.DiagnosticMetrics = &telemetryv1beta1.MetricPipelineIstioInputDiagnosticMetrics{}
	}

	b.inPrometheus.DiagnosticMetrics.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithIstioInputDiagnosticMetrics(enable bool) *MetricPipelineBuilder {
	if b.inIstio == nil {
		b.inIstio = &telemetryv1beta1.MetricPipelineIstioInput{}
	}

	if b.inIstio.DiagnosticMetrics == nil {
		b.inIstio.DiagnosticMetrics = &telemetryv1beta1.MetricPipelineIstioInputDiagnosticMetrics{}
	}

	b.inIstio.DiagnosticMetrics.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputPodMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.Pod == nil {
		b.inRuntime.Resources.Pod = &telemetryv1beta1.MetricPipelineRuntimeInputResource{}
	}

	b.inRuntime.Resources.Pod.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputContainerMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.Container == nil {
		b.inRuntime.Resources.Container = &telemetryv1beta1.MetricPipelineRuntimeInputResource{}
	}

	b.inRuntime.Resources.Container.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputNodeMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.Node == nil {
		b.inRuntime.Resources.Node = &telemetryv1beta1.MetricPipelineRuntimeInputResource{}
	}

	b.inRuntime.Resources.Node.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputVolumeMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.Volume == nil {
		b.inRuntime.Resources.Volume = &telemetryv1beta1.MetricPipelineRuntimeInputResource{}
	}

	b.inRuntime.Resources.Volume.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputDeploymentMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.Deployment == nil {
		b.inRuntime.Resources.Deployment = &telemetryv1beta1.MetricPipelineRuntimeInputResource{}
	}

	b.inRuntime.Resources.Deployment.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputJobMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.Job == nil {
		b.inRuntime.Resources.Job = &telemetryv1beta1.MetricPipelineRuntimeInputResource{}
	}

	b.inRuntime.Resources.Job.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputDaemonSetMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.DaemonSet == nil {
		b.inRuntime.Resources.DaemonSet = &telemetryv1beta1.MetricPipelineRuntimeInputResource{}
	}

	b.inRuntime.Resources.DaemonSet.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputStatefulSetMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.StatefulSet == nil {
		b.inRuntime.Resources.StatefulSet = &telemetryv1beta1.MetricPipelineRuntimeInputResource{}
	}

	b.inRuntime.Resources.StatefulSet.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithOTLPOutput(opts ...OTLPOutputOption) *MetricPipelineBuilder {
	for _, opt := range opts {
		opt(b.outOTLP)
	}

	return b
}

func (b *MetricPipelineBuilder) WithOAuth2(opts ...OAuth2Option) *MetricPipelineBuilder {
	if b.oauth2 == nil {
		b.oauth2 = &telemetryv1beta1.OAuth2Options{}
	}

	for _, opt := range opts {
		opt(b.oauth2)
	}

	// Set OAuth2 on the OTLP output authentication
	if b.outOTLP.Authentication == nil {
		b.outOTLP.Authentication = &telemetryv1beta1.AuthenticationOptions{}
	}

	b.outOTLP.Authentication.OAuth2 = b.oauth2

	return b
}

func (b *MetricPipelineBuilder) WithTransform(transform telemetryv1beta1.TransformSpec) *MetricPipelineBuilder {
	b.transforms = append(b.transforms, transform)
	return b
}

func (b *MetricPipelineBuilder) WithFilter(filter telemetryv1beta1.FilterSpec) *MetricPipelineBuilder {
	b.filter = append(b.filter, filter)
	return b
}

func (b *MetricPipelineBuilder) WithStatusCondition(cond metav1.Condition) *MetricPipelineBuilder {
	b.statusConditions = append(b.statusConditions, cond)
	return b
}

func (b *MetricPipelineBuilder) Build() telemetryv1beta1.MetricPipeline {
	name := b.name
	if name == "" {
		name = fmt.Sprintf("test-%d", b.randSource.Int63())
	}

	pipeline := telemetryv1beta1.MetricPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      b.labels,
			Annotations: b.annotations,
		},
		Status: telemetryv1beta1.MetricPipelineStatus{
			Conditions: b.statusConditions,
		},
		Spec: telemetryv1beta1.MetricPipelineSpec{
			Input: telemetryv1beta1.MetricPipelineInput{
				Runtime:    b.inRuntime,
				Prometheus: b.inPrometheus,
				Istio:      b.inIstio,
				OTLP:       b.inOTLP,
			},
			Output: telemetryv1beta1.MetricPipelineOutput{
				OTLP: b.outOTLP,
			},
			Transforms: b.transforms,
			Filters:    b.filter,
		},
	}

	return pipeline
}

func (b *MetricPipelineBuilder) WithIstioInputEnvoyMetrics(enable bool) *MetricPipelineBuilder {
	if b.inIstio == nil {
		b.inIstio = &telemetryv1beta1.MetricPipelineIstioInput{}
	}

	if b.inIstio.EnvoyMetrics == nil {
		b.inIstio.EnvoyMetrics = &telemetryv1beta1.EnvoyMetrics{}
	}

	b.inIstio.EnvoyMetrics.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) initializeRuntimeInputResources() {
	if b.inRuntime == nil {
		b.inRuntime = &telemetryv1beta1.MetricPipelineRuntimeInput{}
	}

	if b.inRuntime.Resources == nil {
		b.inRuntime.Resources = &telemetryv1beta1.MetricPipelineRuntimeInputResources{}
	}
}
