package misc

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
	suite.RegisterTestCase(t, suite.LabelMetricsMisc)

	const (
		endpointKey         = "endpoint"
		invalidEndpoint     = "'http://example.com'"
		missingPortEndpoint = "http://example.com:/"
	)

	var (
		uniquePrefix                = unique.Prefix()
		pipelineNameValue           = uniquePrefix("value")
		pipelineNameValueFromSecret = uniquePrefix("value-from-secret")
		pipelineNameMissingPortHTTP = uniquePrefix("missing-port-http")
		secretName                  = uniquePrefix()
	)

	pipelineInvalidEndpoint := testutils.NewMetricPipelineBuilder().
		WithName(pipelineNameValue).
		WithOTLPOutput(
			testutils.OTLPEndpoint(invalidEndpoint),
		).
		Build()

	secret := kitk8sobjects.NewOpaqueSecret(secretName, kitkyma.DefaultNamespaceName, kitk8sobjects.WithStringData(endpointKey, invalidEndpoint))
	pipelineInvalidEndpointValueFrom := testutils.NewMetricPipelineBuilder().
		WithName(pipelineNameValueFromSecret).
		WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), secret.Namespace(), endpointKey)).
		Build()

	pipelineMissingPortHTTP := testutils.NewMetricPipelineBuilder().
		WithName(pipelineNameMissingPortHTTP).
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

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.MetricPipelineHasCondition(t, pipelineNameValueFromSecret, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonEndpointInvalid,
	})

	assert.MetricPipelineHasCondition(t, pipelineNameValue, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonEndpointInvalid,
	})

	t.Log("Should set ConfigurationGenerated condition to True in pipelines with missing port and HTTP protocol")
	assert.MetricPipelineHasCondition(t, pipelineNameMissingPortHTTP, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionTrue,
		Reason: conditions.ReasonGatewayConfigured,
	})
}
