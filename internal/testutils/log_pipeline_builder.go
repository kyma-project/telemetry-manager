package testutils

import (
	"fmt"
	"math/rand"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type LogPipelineBuilder struct {
	randSource rand.Source

	name             string
	statusConditions []metav1.Condition
	tlsCert          string
	tlsKey           string
}

func NewLogPipelineBuilder() *LogPipelineBuilder {
	return &LogPipelineBuilder{
		randSource: rand.NewSource(time.Now().UnixNano()),
	}
}

func (b *LogPipelineBuilder) WithName(name string) *LogPipelineBuilder {
	b.name = name
	return b
}

func (b *LogPipelineBuilder) WithStatusCondition(cond metav1.Condition) *LogPipelineBuilder {
	b.statusConditions = append(b.statusConditions, cond)
	return b
}

func (b *LogPipelineBuilder) WithTLS(tlsCert, tlsKey string) *LogPipelineBuilder {
	b.tlsCert = tlsCert
	b.tlsKey = tlsKey
	return b
}

func (b *LogPipelineBuilder) WithTLSOutPut() telemetryv1alpha1.Output {
	return telemetryv1alpha1.Output{
		HTTP: &telemetryv1alpha1.HTTPOutput{
			TLSConfig: telemetryv1alpha1.TLSConfig{
				Cert: &telemetryv1alpha1.ValueType{Value: b.tlsCert},
				Key:  &telemetryv1alpha1.ValueType{Value: b.tlsKey},
			},
		},
	}
}

func (b *LogPipelineBuilder) Build() telemetryv1alpha1.LogPipeline {
	name := b.name
	if name == "" {
		name = fmt.Sprintf("test-%d", b.randSource.Int63())
	}
	logPipeline := telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{},
		Status: telemetryv1alpha1.LogPipelineStatus{
			Conditions: b.statusConditions,
		},
	}

	if b.isTLSEnabled() {
		logPipeline.Spec.Output = b.WithTLSOutPut()
	}
	
	return logPipeline
}

func (b *LogPipelineBuilder) isTLSEnabled() bool {
	return b.tlsCert != "" && b.tlsKey != ""
}
