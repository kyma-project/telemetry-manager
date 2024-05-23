package testutils

import (
	"fmt"
	"math/rand"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type TracePipelineBuilder struct {
	randSource rand.Source

	name   string
	labels map[string]string

	statusConditions []metav1.Condition
	outOTLP          *telemetryv1alpha1.OtlpOutput
}

func NewTracePipelineBuilder() *TracePipelineBuilder {
	return &TracePipelineBuilder{
		randSource: rand.NewSource(time.Now().UnixNano()),
		outOTLP: &telemetryv1alpha1.OtlpOutput{
			Endpoint: telemetryv1alpha1.ValueType{Value: "https://localhost:4317"},
		},
	}
}

func (b *TracePipelineBuilder) WithName(name string) *TracePipelineBuilder {
	b.name = name
	return b
}

func (b *TracePipelineBuilder) WithLabels(labels map[string]string) *TracePipelineBuilder {
	b.labels = labels
	return b
}

func (b *TracePipelineBuilder) WithStatusCondition(cond metav1.Condition) *TracePipelineBuilder {
	b.statusConditions = append(b.statusConditions, cond)
	return b
}

func (b *TracePipelineBuilder) WithStatusConditions(conds ...metav1.Condition) *TracePipelineBuilder {
	b.statusConditions = append(b.statusConditions, conds...)
	return b
}

func (b *TracePipelineBuilder) WithOTLPOutput(opts ...OTLPOutputOption) *TracePipelineBuilder {
	for _, opt := range opts {
		opt(b.outOTLP)
	}
	return b
}

func (b *TracePipelineBuilder) Build() telemetryv1alpha1.TracePipeline {
	name := b.name
	if name == "" {
		name = fmt.Sprintf("test-%d", b.randSource.Int63())
	}

	pipeline := telemetryv1alpha1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Generation: 1,
			Labels:     b.labels,
		},
		Spec: telemetryv1alpha1.TracePipelineSpec{
			Output: telemetryv1alpha1.TracePipelineOutput{
				Otlp: b.outOTLP,
			},
		},
		Status: telemetryv1alpha1.TracePipelineStatus{
			Conditions: b.statusConditions,
		},
	}

	return pipeline
}
