//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelTraces), func() {
	const (
		invalidEndpoint     = "'http://example.com'"
		invalidEndpointHTTP = "example.com"
	)

	var (
		mockNs                = suite.ID()
		pipelineNameValue     = suite.IDWithSuffix("value")
		pipelineNameValueFrom = suite.IDWithSuffix("value-from")
		pipelineNameHTTP      = suite.IDWithSuffix("invalid-http")
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		tracePipelineInvalidEndpointValue := testutils.NewTracePipelineBuilder().
			WithName(pipelineNameValue).
			WithOTLPOutput(
				testutils.OTLPEndpoint(invalidEndpoint),
			).
			Build()

		endpointKey := "endpoint"
		secret := kitk8s.NewOpaqueSecret("test", kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, invalidEndpoint))
		tracePipelineInvalidEndpointValueFrom := testutils.NewTracePipelineBuilder().
			WithName(pipelineNameValueFrom).
			WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), secret.Namespace(), endpointKey)).
			Build()

		tracePipelineInvalidEndpointHTTP := testutils.NewTracePipelineBuilder().
			WithName(pipelineNameHTTP).
			WithOTLPOutput(
				testutils.OTLPEndpoint(invalidEndpointHTTP),
				testutils.OTLPProtocol("http"),
			).
			Build()

		objs = append(objs,
			secret.K8sObject(),
			&tracePipelineInvalidEndpointValue,
			&tracePipelineInvalidEndpointValueFrom,
			&tracePipelineInvalidEndpointHTTP,
		)

		return objs
	}

	Context("When trace pipelines with an invalid Endpoint are created", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should set ConfigurationGenerated condition to False in pipelines", func() {
			assert.TracePipelineHasCondition(ctx, k8sClient, pipelineNameValue, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonEndpointInvalid,
			})

			assert.TracePipelineHasCondition(ctx, k8sClient, pipelineNameValueFrom, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonEndpointInvalid,
			})

			assert.TracePipelineHasCondition(ctx, k8sClient, pipelineNameHTTP, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonEndpointInvalid,
			})
		})
	})
})
