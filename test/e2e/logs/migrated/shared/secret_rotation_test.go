package shared

import (
	"context"
	"fmt"
	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

func TestSecretRotation_Otel(t *testing.T) {
	RegisterTestingT(t)
	//suite.SkipIfDoesNotMatchLabel(t, "logs")
	tests := []struct {
		name                 string
		logPipelineInputFunc func() telemetryv1alpha1.LogPipelineInput
		agent                bool
	}{
		{
			name: "secret-rotation-otel-gateway",
			logPipelineInputFunc: func() telemetryv1alpha1.LogPipelineInput {
				return telemetryv1alpha1.LogPipelineInput{
					Application: &telemetryv1alpha1.LogPipelineApplicationInput{
						Enabled: ptr.To(false),
					},
				}
			},
		}, {
			name: "secret-rotation-otel-agent",
			logPipelineInputFunc: func() telemetryv1alpha1.LogPipelineInput {
				return telemetryv1alpha1.LogPipelineInput{
					Application: &telemetryv1alpha1.LogPipelineApplicationInput{
						Enabled: ptr.To(true),
					},
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				endpointKey  = "logs-endpoint"
				pipelineName = fmt.Sprintf("%s-pipeline", tc.name)
			)
			secret := kitk8s.NewOpaqueSecret("logs-missing", kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, "http://localhost:4317"))

			logPipelineOutPut := telemetryv1alpha1.LogPipelineOutput{
				OTLP: &telemetryv1alpha1.OTLPOutput{
					Endpoint: telemetryv1alpha1.ValueType{
						ValueFrom: &telemetryv1alpha1.ValueFromSource{
							SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
								Name:      secret.Name(),
								Namespace: secret.Namespace(),
								Key:       endpointKey,
							}},
					},
				},
			}
			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.logPipelineInputFunc()).
				WithOutput(logPipelineOutPut).
				Build()

			var resources []client.Object
			resources = append(resources,
				&pipeline,
			)

			t.Cleanup(func() {
				err := kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)
				require.NoError(t, err)
			})
			Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

			assert.LogPipelineHasCondition(suite.Ctx, suite.K8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonReferencedSecretMissing,
			})

			assert.LogPipelineHasCondition(suite.Ctx, suite.K8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonConfigNotGenerated,
			})

			assert.TelemetryHasState(suite.Ctx, suite.K8sClient, operatorv1alpha1.StateWarning)
			assert.TelemetryHasCondition(suite.Ctx, suite.K8sClient, metav1.Condition{
				Type:   conditions.TypeLogComponentsHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonReferencedSecretMissing,
			})

			Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, secret.K8sObject())).Should(Succeed())
			assert.LogPipelineHealthy(suite.Ctx, suite.K8sClient, pipelineName)

		})
	}
}

func TestSecretRotation_FB(t *testing.T) {
	RegisterTestingT(t)
	//suite.SkipIfDoesNotMatchLabel(t, "logs")

	var (
		endpointKey  = "logs-endpoint"
		pipelineName = "secret=rotation-fluentbit-pipeline"
	)
	secret := kitk8s.NewOpaqueSecret("logs-missing", kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, "http://localhost:4317"))

	logPipelineOutPut := telemetryv1alpha1.LogPipelineOutput{
		HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
			Host: telemetryv1alpha1.ValueType{
				ValueFrom: &telemetryv1alpha1.ValueFromSource{
					SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
						Name:      secret.Name(),
						Namespace: secret.Namespace(),
						Key:       endpointKey,
					}},
			},
		},
	}
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithInput(telemetryv1alpha1.LogPipelineInput{
			Application: &telemetryv1alpha1.LogPipelineApplicationInput{
				Enabled: ptr.To(true),
			},
		}).
		WithOutput(logPipelineOutPut).
		Build()

	var resources []client.Object
	resources = append(resources,
		&pipeline,
	)

	assert.LogPipelineHasCondition(suite.Ctx, suite.K8sClient, pipelineName, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonReferencedSecretMissing,
	})

	assert.LogPipelineHasCondition(suite.Ctx, suite.K8sClient, pipelineName, metav1.Condition{
		Type:   conditions.TypeFlowHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonSelfMonConfigNotGenerated,
	})

	assert.TelemetryHasState(suite.Ctx, suite.K8sClient, operatorv1alpha1.StateWarning)
	assert.TelemetryHasCondition(suite.Ctx, suite.K8sClient, metav1.Condition{
		Type:   conditions.TypeLogComponentsHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonReferencedSecretMissing,
	})

	Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, secret.K8sObject())).Should(Succeed())
	assert.LogPipelineHealthy(suite.Ctx, suite.K8sClient, pipelineName)

}
