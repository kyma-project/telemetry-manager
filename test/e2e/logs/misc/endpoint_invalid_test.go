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

func TestEndpointInvalid_OTel(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelLogsMisc)

	const (
		endpointKey     = "endpoint"
		invalidEndpoint = "'http://example.com'"
	)

	var (
		uniquePrefix                = unique.Prefix()
		pipelineNameValue           = uniquePrefix("value")
		pipelineNameValueFromSecret = uniquePrefix("value-from-secret")
		secretName                  = uniquePrefix()
	)

	pipelineInvalidEndpointValue := testutils.NewLogPipelineBuilder().
		WithName(pipelineNameValue).
		WithOTLPOutput(testutils.OTLPEndpoint(invalidEndpoint)).
		Build()

	secret := kitk8sobjects.NewOpaqueSecret(secretName, kitkyma.DefaultNamespaceName, kitk8sobjects.WithStringData(endpointKey, invalidEndpoint))
	pipelineInvalidEndpointValueFrom := testutils.NewLogPipelineBuilder().
		WithName(pipelineNameValueFromSecret).
		WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), secret.Namespace(), endpointKey)).
		Build()

	resourcesToSucceedCreation := []client.Object{
		secret.K8sObject(),
		&pipelineInvalidEndpointValueFrom,
		&pipelineInvalidEndpointValue,
	}

	Expect(kitk8s.CreateObjects(t, resourcesToSucceedCreation...)).To(Succeed())

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

	secret := kitk8sobjects.NewOpaqueSecret(secretName, kitkyma.DefaultNamespaceName, kitk8sobjects.WithStringData(hostKey, invalidHost))
	pipelineInvalidEndpointValueFrom := testutils.NewLogPipelineBuilder().
		WithName(pipelineNameValueFromSecret).
		WithHTTPOutput(testutils.HTTPHostFromSecret(secret.Name(), secret.Namespace(), hostKey)).
		Build()

	resourcesInvalidEndpointValueFrom := []client.Object{
		secret.K8sObject(),
		&pipelineInvalidEndpointValueFrom,
	}

	resourcesInvalidEndpointValue := []client.Object{
		&pipelineInvalidEndpointValue,
	}

	Expect(kitk8s.CreateObjects(t, resourcesInvalidEndpointValueFrom...)).To(Succeed())
	Expect(kitk8s.CreateObjects(t, resourcesInvalidEndpointValue...)).To(Succeed())

	assert.LogPipelineHasCondition(t, pipelineNameValueFromSecret, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonEndpointInvalid,
	})

	assert.LogPipelineHasCondition(t, pipelineNameValueFromSecret, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonEndpointInvalid,
	})
}
