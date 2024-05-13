//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelTraces), func() {
	Context("When tracepipeline with missing secret reference exists", Ordered, func() {
		var pipelineName = suite.ID()

		secretName, secretNamespace, endpointKey := "backend", kitkyma.DefaultNamespaceName, "trace-endpoint"
		secret := kitk8s.NewOpaqueSecret(secretName, secretNamespace, kitk8s.WithStringData(endpointKey, "http://localhost:4317"))
		tracePipeline := testutils.NewTracePipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpointFromSecret(secretName, secretNamespace, endpointKey)).
			Build()

		BeforeAll(func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, &tracePipeline)).Should(Succeed())

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, &tracePipeline, secret.K8sObject())).Should(Succeed())
			})
		})

		It("Should set ConfigurationGenerated condition to false and Pending condition to true", func() {
			assert.TracePipelineHasCondition(ctx, k8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonReferencedSecretMissing,
			})

			assert.TracePipelineHasCondition(ctx, k8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypePending,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonReferencedSecretMissing,
			})
		})

		It("Should not have trace gateway deployment", func() {
			Eventually(func(g Gomega) bool {
				var deployment appsv1.Deployment
				err := k8sClient.Get(ctx, kitkyma.TraceGatewayName, &deployment)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue())
		})

		It("Should have running tracepipeline", func() {
			By("Creating missing secret", func() {
				Expect(kitk8s.CreateObjects(ctx, k8sClient, secret.K8sObject())).Should(Succeed())
			})

			assert.TracePipelineHealthy(ctx, k8sClient, pipelineName)
		})
	})

})
