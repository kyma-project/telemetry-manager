//go:build e2e

package traces

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelTraces), func() {
	const (
		invalidEndpoint     = "'http://example.com'"
		invalidEndpointHTTP = "example.com"
	)

	var (
		mockNs                = ID()
		pipelineNameValue     = IDWithSuffix("value")
		pipelineNameValueFrom = IDWithSuffix("value-from")
		pipelineNameHTTP      = IDWithSuffix("invalid-http")
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
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should set ConfigurationGenerated condition to False in pipelines", func() {
			assert.TracePipelineHasCondition(Ctx, K8sClient, pipelineNameValue, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonEndpointInvalid,
			})

			assert.TracePipelineHasCondition(Ctx, K8sClient, pipelineNameValueFrom, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonEndpointInvalid,
			})

			assert.TracePipelineHasCondition(Ctx, K8sClient, pipelineNameHTTP, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonEndpointInvalid,
			})
		})
	})
})
