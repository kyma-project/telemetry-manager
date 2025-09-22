package test

import (
	"fmt"
	"math/rand"
	"time"

	"k8s.io/utils/ptr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type MetricPipelineBuilder struct {
	randSource rand.Source

	name        string
	labels      map[string]string
	annotations map[string]string

	inRuntime    *telemetryv1alpha1.MetricPipelineRuntimeInput
	inPrometheus *telemetryv1alpha1.MetricPipelinePrometheusInput
	inIstio      *telemetryv1alpha1.MetricPipelineIstioInput
	inOTLP       *telemetryv1alpha1.OTLPInput

	outOTLP *telemetryv1alpha1.OTLPOutput

	transforms       []telemetryv1alpha1.TransformSpec
	statusConditions []metav1.Condition
}

func NewMetricPipelineBuilder() *MetricPipelineBuilder {
	return &MetricPipelineBuilder{
		randSource: rand.NewSource(time.Now().UnixNano()),
		outOTLP: &telemetryv1alpha1.OTLPOutput{
			Endpoint: telemetryv1alpha1.ValueType{Value: "http://localhost:4317"},
		},
	}
}

func buildMetricPipelineInput(
	enablePrometheus bool, prometheusOpts []NamespaceSelectorOptions,
	enableRuntime bool, runtimeOpts []NamespaceSelectorOptions,
	enableIstio bool, istioOpts []NamespaceSelectorOptions,
	enableOTLP bool, otlpOpts []NamespaceSelectorOptions,
) telemetryv1alpha1.MetricPipelineInput {
	input := telemetryv1alpha1.MetricPipelineInput{}

	if enablePrometheus {
		input.Prometheus = &telemetryv1alpha1.MetricPipelinePrometheusInput{
			Enabled:    ptr.To(true),
			Namespaces: &telemetryv1alpha1.NamespaceSelector{},
		}
		for _, opt := range prometheusOpts {
			opt(input.Prometheus.Namespaces)
		}
	} else {
		input.Prometheus = &telemetryv1alpha1.MetricPipelinePrometheusInput{
			Enabled: ptr.To(false),
		}
	}

	if enableRuntime {
		input.Runtime = &telemetryv1alpha1.MetricPipelineRuntimeInput{
			Enabled:    ptr.To(true),
			Namespaces: &telemetryv1alpha1.NamespaceSelector{},
		}
		for _, opt := range runtimeOpts {
			opt(input.Runtime.Namespaces)
		}
	} else {
		input.Runtime = &telemetryv1alpha1.MetricPipelineRuntimeInput{
			Enabled: ptr.To(false),
		}
	}

	if enableIstio {
		input.Istio = &telemetryv1alpha1.MetricPipelineIstioInput{
			Enabled:    ptr.To(true),
			Namespaces: &telemetryv1alpha1.NamespaceSelector{},
		}
		for _, opt := range istioOpts {
			opt(input.Istio.Namespaces)
		}
	} else {
		input.Istio = &telemetryv1alpha1.MetricPipelineIstioInput{
			Enabled: ptr.To(false),
		}
	}

	if enableOTLP {
		input.OTLP = &telemetryv1alpha1.OTLPInput{
			Disabled:   false,
			Namespaces: &telemetryv1alpha1.NamespaceSelector{},
		}
		for _, opt := range otlpOpts {
			opt(input.OTLP.Namespaces)
		}
	} else {
		input.OTLP = &telemetryv1alpha1.OTLPInput{
			Disabled: true,
		}
	}

	return input
}

func BuildMetricPipelineNoInput() telemetryv1alpha1.MetricPipelineInput {
	return buildMetricPipelineInput(
		false, nil,
		false, nil,
		false, nil,
		false, nil,
	)
}

func BuildMetricPipelinePrometheusInput(opts ...NamespaceSelectorOptions) telemetryv1alpha1.MetricPipelineInput {
	return buildMetricPipelineInput(
		true, opts,
		false, nil,
		false, nil,
		false, nil,
	)
}

func BuildMetricPipelineRuntimeInput(opts ...NamespaceSelectorOptions) telemetryv1alpha1.MetricPipelineInput {
	return buildMetricPipelineInput(
		false, nil,
		true, opts,
		false, nil,
		false, nil,
	)
}

func BuildMetricPipelineIstioInput(opts ...NamespaceSelectorOptions) telemetryv1alpha1.MetricPipelineInput {
	return buildMetricPipelineInput(
		false, nil,
		false, nil,
		true, opts,
		false, nil,
	)
}

func BuildMetricPipelineOTLPInput(opts ...NamespaceSelectorOptions) telemetryv1alpha1.MetricPipelineInput {
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

func (b *MetricPipelineBuilder) WithInput(input telemetryv1alpha1.MetricPipelineInput) *MetricPipelineBuilder {
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
		b.inRuntime = &telemetryv1alpha1.MetricPipelineRuntimeInput{}
	}

	b.inRuntime.Enabled = &enable

	if len(opts) == 0 {
		return b
	}

	if b.inRuntime.Namespaces == nil {
		b.inRuntime.Namespaces = &telemetryv1alpha1.NamespaceSelector{}
	}

	for _, opt := range opts {
		opt(b.inRuntime.Namespaces)
	}

	return b
}

func (b *MetricPipelineBuilder) WithPrometheusInput(enable bool, opts ...NamespaceSelectorOptions) *MetricPipelineBuilder {
	if b.inPrometheus == nil {
		b.inPrometheus = &telemetryv1alpha1.MetricPipelinePrometheusInput{}
	}

	b.inPrometheus.Enabled = &enable

	if len(opts) == 0 {
		return b
	}

	if b.inPrometheus.Namespaces == nil {
		b.inPrometheus.Namespaces = &telemetryv1alpha1.NamespaceSelector{}
	}

	for _, opt := range opts {
		opt(b.inPrometheus.Namespaces)
	}

	return b
}

func (b *MetricPipelineBuilder) WithIstioInput(enable bool, opts ...NamespaceSelectorOptions) *MetricPipelineBuilder {
	if b.inIstio == nil {
		b.inIstio = &telemetryv1alpha1.MetricPipelineIstioInput{}
	}

	b.inIstio.Enabled = &enable

	if len(opts) == 0 {
		return b
	}

	if b.inIstio.Namespaces == nil {
		b.inIstio.Namespaces = &telemetryv1alpha1.NamespaceSelector{}
	}

	for _, opt := range opts {
		opt(b.inIstio.Namespaces)
	}

	return b
}

func (b *MetricPipelineBuilder) WithOTLPInput(enable bool, opts ...NamespaceSelectorOptions) *MetricPipelineBuilder {
	if b.inOTLP == nil {
		b.inOTLP = &telemetryv1alpha1.OTLPInput{}
	}

	b.inOTLP.Disabled = !enable

	if len(opts) == 0 {
		return b
	}

	if b.inOTLP.Namespaces == nil {
		b.inOTLP.Namespaces = &telemetryv1alpha1.NamespaceSelector{}
	}

	for _, opt := range opts {
		opt(b.inOTLP.Namespaces)
	}

	return b
}

func (b *MetricPipelineBuilder) WithPrometheusInputDiagnosticMetrics(enable bool) *MetricPipelineBuilder {
	if b.inPrometheus == nil {
		b.inPrometheus = &telemetryv1alpha1.MetricPipelinePrometheusInput{}
	}

	if b.inPrometheus.DiagnosticMetrics == nil {
		b.inPrometheus.DiagnosticMetrics = &telemetryv1alpha1.MetricPipelineIstioInputDiagnosticMetrics{}
	}

	b.inPrometheus.DiagnosticMetrics.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithIstioInputDiagnosticMetrics(enable bool) *MetricPipelineBuilder {
	if b.inIstio == nil {
		b.inIstio = &telemetryv1alpha1.MetricPipelineIstioInput{}
	}

	if b.inIstio.DiagnosticMetrics == nil {
		b.inIstio.DiagnosticMetrics = &telemetryv1alpha1.MetricPipelineIstioInputDiagnosticMetrics{}
	}

	b.inIstio.DiagnosticMetrics.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputPodMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.Pod == nil {
		b.inRuntime.Resources.Pod = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{}
	}

	b.inRuntime.Resources.Pod.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputContainerMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.Container == nil {
		b.inRuntime.Resources.Container = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{}
	}

	b.inRuntime.Resources.Container.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputNodeMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.Node == nil {
		b.inRuntime.Resources.Node = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{}
	}

	b.inRuntime.Resources.Node.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputVolumeMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.Volume == nil {
		b.inRuntime.Resources.Volume = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{}
	}

	b.inRuntime.Resources.Volume.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputDeploymentMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.Deployment == nil {
		b.inRuntime.Resources.Deployment = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{}
	}

	b.inRuntime.Resources.Deployment.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputJobMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.Job == nil {
		b.inRuntime.Resources.Job = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{}
	}

	b.inRuntime.Resources.Job.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputDaemonSetMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.DaemonSet == nil {
		b.inRuntime.Resources.DaemonSet = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{}
	}

	b.inRuntime.Resources.DaemonSet.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputStatefulSetMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.StatefulSet == nil {
		b.inRuntime.Resources.StatefulSet = &telemetryv1alpha1.MetricPipelineRuntimeInputResource{}
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

func (b *MetricPipelineBuilder) WithTransform(transform telemetryv1alpha1.TransformSpec) *MetricPipelineBuilder {
	b.transforms = append(b.transforms, transform)
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
			Name:        name,
			Labels:      b.labels,
			Annotations: b.annotations,
		},
		Status: telemetryv1alpha1.MetricPipelineStatus{
			Conditions: b.statusConditions,
		},
		Spec: telemetryv1alpha1.MetricPipelineSpec{
			Input: telemetryv1alpha1.MetricPipelineInput{
				Runtime:    b.inRuntime,
				Prometheus: b.inPrometheus,
				Istio:      b.inIstio,
				OTLP:       b.inOTLP,
			},
			Output: telemetryv1alpha1.MetricPipelineOutput{
				OTLP: b.outOTLP,
			},
			Transforms: b.transforms,
		},
	}

	return pipeline
}

func (b *MetricPipelineBuilder) WithIstioInputEnvoyMetrics(enable bool) *MetricPipelineBuilder {
	if b.inIstio == nil {
		b.inIstio = &telemetryv1alpha1.MetricPipelineIstioInput{}
	}

	if b.inIstio.EnvoyMetrics == nil {
		b.inIstio.EnvoyMetrics = &telemetryv1alpha1.EnvoyMetrics{}
	}

	b.inIstio.EnvoyMetrics.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) initializeRuntimeInputResources() {
	if b.inRuntime == nil {
		b.inRuntime = &telemetryv1alpha1.MetricPipelineRuntimeInput{}
	}

	if b.inRuntime.Resources == nil {
		b.inRuntime.Resources = &telemetryv1alpha1.MetricPipelineRuntimeInputResources{}
	}
}
