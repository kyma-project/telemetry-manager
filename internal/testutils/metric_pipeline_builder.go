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

	name        string
	labels      map[string]string
	annotations map[string]string

	inRuntime    *telemetryv1alpha1.MetricPipelineRuntimeInput
	inPrometheus *telemetryv1alpha1.MetricPipelinePrometheusInput
	inIstio      *telemetryv1alpha1.MetricPipelineIstioInput
	inOTLP       *telemetryv1alpha1.OTLPInput

	outOTLP *telemetryv1alpha1.OTLPOutput

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

type InputOptions func(selector *telemetryv1alpha1.NamespaceSelector)

func IncludeNamespaces(namespaces ...string) InputOptions {
	return func(selector *telemetryv1alpha1.NamespaceSelector) {
		selector.Include = namespaces
		selector.Exclude = nil
	}
}

func ExcludeNamespaces(namespaces ...string) InputOptions {
	return func(selector *telemetryv1alpha1.NamespaceSelector) {
		selector.Include = nil
		selector.Exclude = namespaces
	}
}

func (b *MetricPipelineBuilder) WithRuntimeInput(enable bool, opts ...InputOptions) *MetricPipelineBuilder {
	if b.inRuntime == nil {
		b.inRuntime = &telemetryv1alpha1.MetricPipelineRuntimeInput{}
	}

	b.inRuntime.Enabled = enable

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

func (b *MetricPipelineBuilder) WithPrometheusInput(enable bool, opts ...InputOptions) *MetricPipelineBuilder {
	if b.inPrometheus == nil {
		b.inPrometheus = &telemetryv1alpha1.MetricPipelinePrometheusInput{}
	}

	b.inPrometheus.Enabled = enable

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

func (b *MetricPipelineBuilder) WithIstioInput(enable bool, opts ...InputOptions) *MetricPipelineBuilder {
	if b.inIstio == nil {
		b.inIstio = &telemetryv1alpha1.MetricPipelineIstioInput{}
	}

	b.inIstio.Enabled = enable

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

func (b *MetricPipelineBuilder) WithOTLPInput(enable bool, opts ...InputOptions) *MetricPipelineBuilder {
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

	b.inPrometheus.DiagnosticMetrics.Enabled = enable

	return b
}

func (b *MetricPipelineBuilder) WithIstioInputDiagnosticMetrics(enable bool) *MetricPipelineBuilder {
	if b.inIstio == nil {
		b.inIstio = &telemetryv1alpha1.MetricPipelineIstioInput{}
	}

	if b.inIstio.DiagnosticMetrics == nil {
		b.inIstio.DiagnosticMetrics = &telemetryv1alpha1.MetricPipelineIstioInputDiagnosticMetrics{}
	}

	b.inIstio.DiagnosticMetrics.Enabled = enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputPodMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.Pod == nil {
		b.inRuntime.Resources.Pod = &telemetryv1alpha1.MetricPipelineRuntimeInputResourceEnabledByDefault{}
	}

	b.inRuntime.Resources.Pod.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputContainerMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.Container == nil {
		b.inRuntime.Resources.Container = &telemetryv1alpha1.MetricPipelineRuntimeInputResourceEnabledByDefault{}
	}

	b.inRuntime.Resources.Container.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputNodeMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.Node == nil {
		b.inRuntime.Resources.Node = &telemetryv1alpha1.MetricPipelineRuntimeInputResourceDisabledByDefault{}
	}

	b.inRuntime.Resources.Node.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputVolumeMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.Volume == nil {
		b.inRuntime.Resources.Volume = &telemetryv1alpha1.MetricPipelineRuntimeInputResourceDisabledByDefault{}
	}

	b.inRuntime.Resources.Volume.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputDeploymentMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.Deployment == nil {
		b.inRuntime.Resources.Deployment = &telemetryv1alpha1.MetricPipelineRuntimeInputResourceDisabledByDefault{}
	}

	b.inRuntime.Resources.Deployment.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputJobMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.Job == nil {
		b.inRuntime.Resources.Job = &telemetryv1alpha1.MetricPipelineRuntimeInputResourceDisabledByDefault{}
	}

	b.inRuntime.Resources.Job.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputDaemonSetMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.DaemonSet == nil {
		b.inRuntime.Resources.DaemonSet = &telemetryv1alpha1.MetricPipelineRuntimeInputResourceDisabledByDefault{}
	}

	b.inRuntime.Resources.DaemonSet.Enabled = &enable

	return b
}

func (b *MetricPipelineBuilder) WithRuntimeInputStatefulSetMetrics(enable bool) *MetricPipelineBuilder {
	b.initializeRuntimeInputResources()

	if b.inRuntime.Resources.StatefulSet == nil {
		b.inRuntime.Resources.StatefulSet = &telemetryv1alpha1.MetricPipelineRuntimeInputResourceDisabledByDefault{}
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
		},
	}

	return pipeline
}

func (b *MetricPipelineBuilder) initializeRuntimeInputResources() {
	if b.inRuntime == nil {
		b.inRuntime = &telemetryv1alpha1.MetricPipelineRuntimeInput{}
	}

	if b.inRuntime.Resources == nil {
		b.inRuntime.Resources = &telemetryv1alpha1.MetricPipelineRuntimeInputResources{}
	}
}
