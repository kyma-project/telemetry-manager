package traces

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestEndpointInvalid(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTraces)

	const (
		endpointKey         = "endpoint"
		invalidEndpoint     = "'http://example.com'"
		missingPortEndpoint = "http://example.com:/"
		invalidPortEndpoint = "http://example.com:9abc8"
	)

	var (
		uniquePrefix                = unique.Prefix()
		pipelineNameValue           = uniquePrefix("value")
		pipelineNameValueFrom       = uniquePrefix("value-from-secret")
		pipelineNameMissingPortHTTP = uniquePrefix("missing-port-http")
		pipelineNameInvalidPortHTTP = uniquePrefix("invalid-port-http")
		secretName                  = uniquePrefix()
	)

	pipelineInvalidEndpoint := testutils.NewTracePipelineBuilder().
		WithName(pipelineNameValue).
		WithOTLPOutput(
			testutils.OTLPEndpoint(invalidEndpoint),
		).
		Build()

	secret := kitk8sobjects.NewOpaqueSecret(secretName, kitkyma.DefaultNamespaceName, kitk8sobjects.WithStringData(endpointKey, invalidEndpoint))
	pipelineInvalidEndpointValueFrom := testutils.NewTracePipelineBuilder().
		WithName(pipelineNameValueFrom).
		WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), secret.Namespace(), endpointKey)).
		Build()

	pipelineMissingPortHTTP := testutils.NewTracePipelineBuilder().
		WithName(pipelineNameMissingPortHTTP).
		WithOTLPOutput(
			testutils.OTLPEndpoint(missingPortEndpoint),
			testutils.OTLPProtocol("http"),
		).
		Build()

	pipelineInvalidPortHTTP := testutils.NewTracePipelineBuilder().
		WithName(pipelineNameInvalidPortHTTP).
		WithOTLPOutput(
			testutils.OTLPEndpoint(invalidPortEndpoint),
			testutils.OTLPProtocol("http"),
		).
		Build()

	resources := []client.Object{
		secret.K8sObject(),
		&pipelineInvalidEndpoint,
		&pipelineInvalidEndpointValueFrom,
		&pipelineMissingPortHTTP,
		&pipelineInvalidPortHTTP,
	}

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	for _, pipelineName := range []string{
		pipelineNameValue,
		pipelineNameValueFrom,
		pipelineNameInvalidPortHTTP,
	} {
		assert.TracePipelineHasCondition(t, pipelineName, metav1.Condition{
			Type:   conditions.TypeConfigurationGenerated,
			Status: metav1.ConditionFalse,
			Reason: conditions.ReasonEndpointInvalid,
		})

		assert.TracePipelineHasCondition(t, pipelineName, metav1.Condition{
			Type:   conditions.TypeConfigurationGenerated,
			Status: metav1.ConditionFalse,
			Reason: conditions.ReasonEndpointInvalid,
		})

		assert.TracePipelineHasCondition(t, pipelineName, metav1.Condition{
			Type:   conditions.TypeConfigurationGenerated,
			Status: metav1.ConditionFalse,
			Reason: conditions.ReasonEndpointInvalid,
		})
	}

	t.Log("Should set ConfigurationGenerated condition to True in pipelines with missing port and HTTP protocol")
	assert.TracePipelineHasCondition(t, pipelineNameMissingPortHTTP, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionTrue,
		Reason: conditions.ReasonGatewayConfigured,
	})
}
