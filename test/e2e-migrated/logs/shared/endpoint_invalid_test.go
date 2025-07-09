package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestEndpointInvalid_OTel(t *testing.T) {
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

			const (
				endpointKey     = "endpoint"
				invalidEndpoint = "'http://example.com'"
			)

			var (
				uniquePrefix                = unique.Prefix(tc.label)
				pipelineNameValue           = uniquePrefix("value")
				pipelineNameValueFromSecret = uniquePrefix("value-from-secret")
				secretName                  = uniquePrefix()
			)

			pipelineInvalidEndpointValue := testutils.NewLogPipelineBuilder().
				WithName(pipelineNameValue).
				WithInput(tc.input).
				WithOTLPOutput(testutils.OTLPEndpoint(invalidEndpoint)).
				Build()

			secret := kitk8s.NewOpaqueSecret(secretName, kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, invalidEndpoint))
			pipelineInvalidEndpointValueFrom := testutils.NewLogPipelineBuilder().
				WithName(pipelineNameValueFromSecret).
				WithInput(tc.input).
				WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), secret.Namespace(), endpointKey)).
				Build()

			resourcesToSucceedCreation := []client.Object{
				secret.K8sObject(),
				&pipelineInvalidEndpointValueFrom,
				&pipelineInvalidEndpointValue,
			}

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(t, resourcesToSucceedCreation...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})

			Expect(kitk8s.CreateObjects(t, resourcesToSucceedCreation...)).Should(Succeed())

			assert.LogPipelineHasCondition(t, pipelineNameValueFromSecret, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonEndpointInvalid,
			})

			assert.LogPipelineHasCondition(t, pipelineNameValue, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonEndpointInvalid,
			})
		})
	}
}

func TestEndpointInvalid_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	const (
		hostKey     = "host"
		invalidHost = "'http://example.com'"
	)

	var (
		uniquePrefix                = unique.Prefix()
		pipelineNameValue           = uniquePrefix("value")
		pipelineNameValueFromSecret = uniquePrefix("value-from-secret")
		secretName                  = uniquePrefix()
	)

	pipelineInvalidEndpointValue := testutils.NewLogPipelineBuilder().
		WithName(pipelineNameValue).
		WithHTTPOutput(testutils.HTTPHost(invalidHost)).
		Build()

	secret := kitk8s.NewOpaqueSecret(secretName, kitkyma.DefaultNamespaceName, kitk8s.WithStringData(hostKey, invalidHost))
	pipelineInvalidEndpointValueFrom := testutils.NewLogPipelineBuilder().
		WithName(pipelineNameValueFromSecret).
		WithHTTPOutput(testutils.HTTPHostFromSecret(secret.Name(), secret.Namespace(), hostKey)).
		Build()

	resourcesToSucceedCreation := []client.Object{
		secret.K8sObject(),
		&pipelineInvalidEndpointValueFrom,
	}

	resourcesToFailCreation := []client.Object{
		&pipelineInvalidEndpointValue,
	}

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(t, resourcesToSucceedCreation...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t, resourcesToSucceedCreation...)).Should(Succeed())
	Expect(kitk8s.CreateObjects(t, resourcesToFailCreation...)).Should(MatchError(ContainSubstring("invalid hostname")))

	assert.LogPipelineHasCondition(t, pipelineNameValueFromSecret, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonEndpointInvalid,
	})
}
