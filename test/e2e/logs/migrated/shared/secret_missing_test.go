package shared

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

// FIXME: Currently failing (Secret validation not implemented for OTel)
func TestSecretRotation_OTel(t *testing.T) {
	tests := []struct {
		label string
		input telemetryv1alpha1.LogPipelineInput
	}{
		{
			label: suite.LabelLogAgent,
			input: testutils.BuildLogPipelineApplicationInput(),
		},
		{
			label: suite.LabelLogGateway,
			input: testutils.BuildLogPipelineOTLPInput(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label)

			const endpointKey = "logs-endpoint"

			var (
				uniquePrefix = unique.Prefix(tc.label)
				pipelineName = uniquePrefix()
			)

			secret := kitk8s.NewOpaqueSecret("logs-missing", kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, "http://localhost:4317"))

			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.input).
				WithOTLPOutput(testutils.OTLPEndpointFromSecret(
					secret.Name(),
					secret.Namespace(),
					endpointKey,
				)).
				Build()

			resources := []client.Object{
				&pipeline,
			}

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

			assert.LogPipelineHasCondition(t.Context(), suite.K8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonReferencedSecretMissing,
			})

			assert.LogPipelineHasCondition(t.Context(), suite.K8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonConfigNotGenerated,
			})

			assert.TelemetryHasState(t.Context(), suite.K8sClient, operatorv1alpha1.StateWarning)
			assert.TelemetryHasCondition(t.Context(), suite.K8sClient, metav1.Condition{
				Type:   conditions.TypeLogComponentsHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonReferencedSecretMissing,
			})

			Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, secret.K8sObject())).Should(Succeed())
			assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineName)
		})
	}
}

func TestSecretRotation_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	const endpointKey = "logs-endpoint"

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
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

	resources := []client.Object{
		&pipeline,
	}

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...)).Should(Succeed())

	assert.LogPipelineHasCondition(t.Context(), suite.K8sClient, pipelineName, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonReferencedSecretMissing,
	})

	assert.LogPipelineHasCondition(t.Context(), suite.K8sClient, pipelineName, metav1.Condition{
		Type:   conditions.TypeFlowHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonSelfMonConfigNotGenerated,
	})

	assert.TelemetryHasState(t.Context(), suite.K8sClient, operatorv1alpha1.StateWarning)
	assert.TelemetryHasCondition(t.Context(), suite.K8sClient, metav1.Condition{
		Type:   conditions.TypeLogComponentsHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonReferencedSecretMissing,
	})

	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, secret.K8sObject())).Should(Succeed())
	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipelineName)
}
