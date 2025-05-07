package shared

import (
	"context"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestSecretRotation_OTel(t *testing.T) {
	RegisterTestingT(t)
	// suite.SkipIfDoesNotMatchLabel(t, "logs")
	tests := []struct {
		name         string
		inputBuilder func() telemetryv1alpha1.LogPipelineInput
		agent        bool
	}{
		{
			name: "gateway",
			inputBuilder: func() telemetryv1alpha1.LogPipelineInput {
				return telemetryv1alpha1.LogPipelineInput{
					Application: &telemetryv1alpha1.LogPipelineApplicationInput{
						Enabled: ptr.To(false),
					},
				}
			},
		}, {
			name: "agent",
			inputBuilder: func() telemetryv1alpha1.LogPipelineInput {
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
				uniquePrefix = unique.Prefix(tc.name)
				endpointKey  = "logs-endpoint"
				pipelineName = uniquePrefix("pipeline")
			)

			secret := kitk8s.NewOpaqueSecret("logs-missing", kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, "http://localhost:4317"))

			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.inputBuilder()).
				WithOTLPOutput(testutils.OTLPEndpointFromSecret(
					secret.Name(),
					secret.Namespace(),
					endpointKey,
				)).
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

			t.Log("Waiting for resources to be ready")

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

func TestSecretRotation_FluentBit(t *testing.T) {
	RegisterTestingT(t)
	// suite.SkipIfDoesNotMatchLabel(t, "logs")

	var (
		uniquePrefix = unique.Prefix()
		endpointKey  = "logs-endpoint"
		pipelineName = uniquePrefix("pipeline")
	)

	secret := kitk8s.NewOpaqueSecret("logs-missing", kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, "http://localhost:4317"))

	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithHTTPOutput(testutils.HTTPHostFromSecret(
			secret.Name(),
			secret.Namespace(),
			endpointKey,
		)).
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

	t.Log("Waiting for resources to be ready")

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
