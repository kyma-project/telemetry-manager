package shared

import (
	"testing"

	. "github.com/onsi/gomega"
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

func TestEndpointInvalid(t *testing.T) {
	tests := []struct {
		label string
		input telemetryv1alpha1.MetricPipelineInput
	}{
		{
			label: suite.LabelMetricAgentSetA,
			input: testutils.BuildMetricPipelineRuntimeInput(),
		},
		{
			label: suite.LabelMetricGatewaySetA,
			input: testutils.BuildMetricPipelineOTLPInput(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label)

			const (
				endpointKey         = "endpoint"
				invalidEndpoint     = "'http://example.com'"
				missingPortEndpoint = "http://example.com:/"
			)

			var (
				uniquePrefix                = unique.Prefix()
				pipelineNameValue           = uniquePrefix("value")
				pipelineNameValueFrom       = uniquePrefix("value-from-secret")
				pipelineNameMissingPortHTTP = uniquePrefix("missing-port-http")
				secretName                  = uniquePrefix()
			)

			pipelineInvalidEndpoint := testutils.NewMetricPipelineBuilder().
				WithName(pipelineNameValue).
				WithInput(tc.input).
				WithOTLPOutput(
					testutils.OTLPEndpoint(invalidEndpoint),
				).
				Build()

			secret := kitk8s.NewOpaqueSecret(secretName, kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, invalidEndpoint))
			pipelineInvalidEndpointValueFrom := testutils.NewMetricPipelineBuilder().
				WithName(pipelineNameValueFrom).
				WithInput(tc.input).
				WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), secret.Namespace(), endpointKey)).
				Build()

			pipelineMissingPortHTTP := testutils.NewMetricPipelineBuilder().
				WithName(pipelineNameMissingPortHTTP).
				WithInput(tc.input).
				WithOTLPOutput(
					testutils.OTLPEndpoint(missingPortEndpoint),
					testutils.OTLPProtocol("http"),
				).
				Build()

			resources := []client.Object{
				secret.K8sObject(),
				&pipelineInvalidEndpoint,
				&pipelineInvalidEndpointValueFrom,
				&pipelineMissingPortHTTP,
			}

			t.Cleanup(func() {
				Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
			})
			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			for _, pipelineName := range []string{
				pipelineNameValue,
				pipelineNameValueFrom,
			} {
				assert.MetricPipelineHasCondition(t, pipelineName, metav1.Condition{
					Type:   conditions.TypeConfigurationGenerated,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonEndpointInvalid,
				})

				assert.MetricPipelineHasCondition(t, pipelineName, metav1.Condition{
					Type:   conditions.TypeConfigurationGenerated,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonEndpointInvalid,
				})

				assert.MetricPipelineHasCondition(t, pipelineName, metav1.Condition{
					Type:   conditions.TypeConfigurationGenerated,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonEndpointInvalid,
				})
			}

			t.Log("Should set ConfigurationGenerated condition to True in pipelines with missing port and HTTP protocol")
			assert.MetricPipelineHasCondition(t, pipelineNameMissingPortHTTP, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonGatewayConfigured,
			})
		})
	}
}
