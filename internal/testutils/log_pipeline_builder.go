package testutils

import (
	"fmt"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math/rand"
	"time"
)

type LogPipelineBuilder struct {
	randSource rand.Source

	name string

	conditions []telemetryv1alpha1.LogPipelineCondition
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

func LogPendingCondition(reason string) telemetryv1alpha1.LogPipelineCondition {
	return telemetryv1alpha1.LogPipelineCondition{
		Reason: reason,
		Type:   telemetryv1alpha1.LogPipelinePending,
	}
}

func LogRunningCondition() telemetryv1alpha1.LogPipelineCondition {
	return telemetryv1alpha1.LogPipelineCondition{
		Reason: reconciler.ReasonFluentBitDSReady,
		Type:   telemetryv1alpha1.LogPipelinePending,
	}
}

func (b *LogPipelineBuilder) WithStatusConditions(conditions ...telemetryv1alpha1.LogPipelineCondition) *LogPipelineBuilder {
	b.conditions = conditions
	return b
}

func (b *LogPipelineBuilder) Build() telemetryv1alpha1.LogPipeline {
	name := b.name
	if name == "" {
		name = fmt.Sprintf("test-%d", b.randSource.Int63())
	}
	return telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{},
		Status: telemetryv1alpha1.LogPipelineStatus{
			Conditions: b.conditions,
		},
	}
}
