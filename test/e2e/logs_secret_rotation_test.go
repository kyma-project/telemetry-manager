//go:build e2e

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogs), Ordered, func() {
	var pipelineName = suite.ID()

	Context("When a LogPipeline with missing secret reference exists", Ordered, func() {

		endpointKey := "logs-endpoint"
		secret := kitk8s.NewOpaqueSecret("logs-missing", kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, "http://localhost:4317"))
		logPipeline := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithHTTPOutput(testutils.HTTPHostFromSecret(
				secret.Name(),
				secret.Namespace(),
				endpointKey,
			)).
			Build()

		BeforeAll(func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, &logPipeline)).Should(Succeed())

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, &logPipeline)).Should(Succeed())
			})
		})

		It("Should set ConfigurationGenerated condition to false and Pending condition to true", func() {
			assert.LogPipelineHasCondition(ctx, k8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonReferencedSecretMissing,
			})

			assert.LogPipelineHasCondition(ctx, k8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypePending,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonReferencedSecretMissing,
			})
		})

		It("Should have a fluent-bit-sections ConfigMap", func() {
			Eventually(func(g Gomega) {
				var configMap corev1.ConfigMap
				g.Expect(k8sClient.Get(ctx, kitkyma.FluentBitSectionsConfigMap, &configMap)).To(Succeed())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should not include the LogPipeline with missing secret in fluent-bit-sections configmap", func() {
			Consistently(func(g Gomega) {
				var configMap corev1.ConfigMap
				g.Expect(k8sClient.Get(ctx, kitkyma.FluentBitSectionsConfigMap, &configMap)).To(Succeed())

				g.Expect(configMap.Data).ShouldNot(HaveKey(fmt.Sprintf("%s.conf", pipelineName)))
			}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should have a healthy pipeline and update fluent-bit-sections configmap", func() {
			By("Creating missing secret", func() {
				Expect(kitk8s.CreateObjects(ctx, k8sClient, secret.K8sObject())).Should(Succeed())
			})

			assert.LogPipelineHealthy(ctx, k8sClient, pipelineName)

			Eventually(func(g Gomega) {
				var configMap corev1.ConfigMap
				g.Expect(k8sClient.Get(ctx, kitkyma.FluentBitSectionsConfigMap, &configMap)).To(Succeed())

				g.Expect(configMap.Data).Should(HaveKey(fmt.Sprintf("%s.conf", pipelineName)))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should remove pipeline from fluent-bit-sections configmap", func() {
			By("Deleting the secret", func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, secret.K8sObject())).Should(Succeed())
			})

			Eventually(func(g Gomega) {
				var configMap corev1.ConfigMap
				g.Expect(k8sClient.Get(ctx, kitkyma.FluentBitSectionsConfigMap, &configMap)).To(Succeed())

				g.Expect(configMap.Data).ShouldNot(HaveKey(fmt.Sprintf("%s.conf", pipelineName)))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

	})
})
