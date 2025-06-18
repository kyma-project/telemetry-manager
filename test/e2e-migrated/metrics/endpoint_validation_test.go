package metrics

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestValidateEndpoint(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetrics)

	const (
		endpointKey         = "endpoint"
		invalidEndpoint     = "'http://example.com'"
		invalidPortEndpoint = "http://example.com:9abc8"
		missingPortEndpoint = "http://example.com:/"
	)

	var (
		uniquePrefix                    = unique.Prefix()
		pipelineNameInvalidEndpoint     = uniquePrefix("invalid-endpoint")
		pipelineNameValueFrom           = uniquePrefix("value-from-secret")
		pipelineNameInvalidPortEndpoint = uniquePrefix("invalid-port-endpoint")
		pipelineNameMissingPortEndpoint = uniquePrefix("missing-port-endpoint")
		pipelineNameMissingHTTP         = uniquePrefix("missing-port-http")
		secretName                      = uniquePrefix()
	)

	pipelineInvalidEndpoint := testutils.NewMetricPipelineBuilder().
		WithName(pipelineNameInvalidEndpoint).
		WithOTLPOutput(
			testutils.OTLPEndpoint(invalidEndpoint),
		).
		Build()

	secret := kitk8s.NewOpaqueSecret(secretName, kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, invalidEndpoint))
	pipelineInvalidEndpointValueFrom := testutils.NewMetricPipelineBuilder().
		WithName(pipelineNameValueFrom).
		WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), secret.Namespace(), endpointKey)).
		Build()

	pipelineInvalidPortEndpoint := testutils.NewMetricPipelineBuilder().
		WithName(pipelineNameInvalidPortEndpoint).
		WithOTLPOutput(
			testutils.OTLPEndpoint(invalidPortEndpoint),
		).
		Build()
	pipelineMissingPortEndpoint := testutils.NewMetricPipelineBuilder().
		WithName(pipelineNameMissingPortEndpoint).
		WithOTLPOutput(
			testutils.OTLPEndpoint(missingPortEndpoint),
		).
		Build()

	pipelineMissingPortHTTP := testutils.NewMetricPipelineBuilder().
		WithName(pipelineNameMissingHTTP).
		WithOTLPOutput(
			testutils.OTLPEndpoint(missingPortEndpoint),
			testutils.OTLPProtocol("http"),
		).
		Build()

	var resources []client.Object
	resources = append(resources,
		secret.K8sObject(),
		&pipelineInvalidEndpoint,
		&pipelineInvalidEndpointValueFrom,
		&pipelineInvalidPortEndpoint,
		&pipelineMissingPortEndpoint,
		&pipelineMissingPortHTTP,
	)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

	for _, pipelineName := range []string{
		pipelineNameInvalidEndpoint,
		pipelineNameValueFrom,
		pipelineNameInvalidPortEndpoint,
		pipelineNameMissingPortEndpoint,
	} {
		assert.MetricPipelineHasConditionWithT(t, pipelineName, metav1.Condition{
			Type:   conditions.TypeConfigurationGenerated,
			Status: metav1.ConditionFalse,
			Reason: conditions.ReasonEndpointInvalid,
		})

		assert.MetricPipelineHasConditionWithT(t, pipelineName, metav1.Condition{
			Type:   conditions.TypeConfigurationGenerated,
			Status: metav1.ConditionFalse,
			Reason: conditions.ReasonEndpointInvalid,
		})

		assert.MetricPipelineHasConditionWithT(t, pipelineName, metav1.Condition{
			Type:   conditions.TypeConfigurationGenerated,
			Status: metav1.ConditionFalse,
			Reason: conditions.ReasonEndpointInvalid,
		})
	}

	t.Log("Should set ConfigurationGenerated condition to True in pipelines with missing port and HTTP protocol")
	assert.MetricPipelineHasConditionWithT(t, pipelineNameMissingHTTP, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionTrue,
		Reason: conditions.ReasonGatewayConfigured,
	})
}
