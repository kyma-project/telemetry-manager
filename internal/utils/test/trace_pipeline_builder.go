package test

import (
	"fmt"
	"math/rand"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

type TracePipelineBuilder struct {
	randSource rand.Source

	name   string
	labels map[string]string

	transforms       []telemetryv1beta1.TransformSpec
	filters          []telemetryv1beta1.FilterSpec
	statusConditions []metav1.Condition
	outOTLP          *telemetryv1beta1.OTLPOutput
	oauth2           *telemetryv1beta1.OAuth2Options
}

func NewTracePipelineBuilder() *TracePipelineBuilder {
	return &TracePipelineBuilder{
		randSource: rand.NewSource(time.Now().UnixNano()),
		outOTLP: &telemetryv1beta1.OTLPOutput{
			Endpoint: telemetryv1beta1.ValueType{Value: "https://localhost:4317"},
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

func (b *TracePipelineBuilder) WithOAuth2(opts ...OAuth2Option) *TracePipelineBuilder {
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

func (b *TracePipelineBuilder) WithTransform(transform telemetryv1beta1.TransformSpec) *TracePipelineBuilder {
	b.transforms = append(b.transforms, transform)
	return b
}

func (b *TracePipelineBuilder) WithFilter(filter telemetryv1beta1.FilterSpec) *TracePipelineBuilder {
	b.filters = append(b.filters, filter)
	return b
}

func (b *TracePipelineBuilder) Build() telemetryv1beta1.TracePipeline {
	name := b.name
	if name == "" {
		name = fmt.Sprintf("test-%d", b.randSource.Int63())
	}

	pipeline := telemetryv1beta1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Generation: 1,
			Labels:     b.labels,
		},
		Spec: telemetryv1beta1.TracePipelineSpec{
			Output: telemetryv1beta1.TracePipelineOutput{
				OTLP: b.outOTLP,
			},
			Transforms: b.transforms,
			Filters:    b.filters,
		},
		Status: telemetryv1beta1.TracePipelineStatus{
			Conditions: b.statusConditions,
		},
	}

	return pipeline
}
