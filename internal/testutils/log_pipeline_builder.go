package testutils

import (
	"fmt"
	"math/rand"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
)

type LogPipelineBuilder struct {
	randSource rand.Source

	name string

	conditions []metav1.Condition
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

func LogPendingCondition(reason string) metav1.Condition {
	return metav1.Condition{
		Type:    conditions.TypePending,
		Status:  metav1.ConditionTrue,
		Reason:  reason,
		Message: conditions.CommonMessageFor(reason),
	}
}

func LogRunningCondition() metav1.Condition {
	return metav1.Condition{
		Type:    conditions.TypeRunning,
		Status:  metav1.ConditionTrue,
		Reason:  conditions.ReasonFluentBitDSReady,
		Message: conditions.CommonMessageFor(conditions.ReasonFluentBitDSReady),
	}
}

func (b *LogPipelineBuilder) WithStatusConditions(conditions ...metav1.Condition) *LogPipelineBuilder {
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
