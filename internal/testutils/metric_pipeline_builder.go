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
	runtimeInputOn    bool
	prometheusInputOn bool
	istioInputOn      bool
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

func (b *MetricPipelineBuilder) WithRuntimeInputOn(on bool) *MetricPipelineBuilder {
	b.runtimeInputOn = on
	return b
}

func (b *MetricPipelineBuilder) WithPrometheusInputOn(on bool) *MetricPipelineBuilder {
	b.prometheusInputOn = on
	return b
}

func (b *MetricPipelineBuilder) WithIstioInputOn(on bool) *MetricPipelineBuilder {
	b.istioInputOn = on
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
	return telemetryv1alpha1.MetricPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: telemetryv1alpha1.MetricPipelineSpec{
			Input: telemetryv1alpha1.MetricPipelineInput{
				Runtime: telemetryv1alpha1.MetricPipelineRuntimeInput{
					Enabled: &b.runtimeInputOn,
				},
				Prometheus: telemetryv1alpha1.MetricPipelinePrometheusInput{
					Enabled: &b.prometheusInputOn,
				},
				Istio: telemetryv1alpha1.MetricPipelineIstioInput{
					Enabled: &b.istioInputOn,
				},
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
}
