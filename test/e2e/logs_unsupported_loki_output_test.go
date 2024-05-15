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

var _ = Describe(suite.ID(), Label(suite.LabelLogs, "test"), Ordered, func() {
	var pipelineName = suite.ID()

	Context("When a LogPipeline with Loki output exists", Ordered, func() {

		BeforeAll(func() {
			pipeline := testutils.NewLogPipelineBuilder().WithName(pipelineName).WithLokiOutput().Build()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, &pipeline)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, &pipeline)).Should(Succeed())
		})

		It("Should have ConfigurationGenerated condition set to False in pipeline", func() {
			assert.LogPipelineHasCondition(ctx, k8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonUnsupportedLokiOutput,
			})
		})

		It("Should have Pending condition set to True in pipeline", func() {
			assert.LogPipelineHasCondition(ctx, k8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypePending,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonUnsupportedLokiOutput,
			})
		})

		It("Should have LogComponentsHealthy condition set to False in Telemetry", func() {
			assert.TelemetryHasCondition(ctx, k8sClient, metav1.Condition{
				Type:   "LogComponentsHealthy",
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonUnsupportedLokiOutput,
			})
		})

		It("Should have a fluent-bit-sections ConfigMap", func() {
			Eventually(func(g Gomega) {
				var configMap corev1.ConfigMap
				g.Expect(k8sClient.Get(ctx, kitkyma.FluentBitSectionsConfigMap, &configMap)).To(Succeed())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should not include the pipeline in fluent-bit-sections ConfigMap", func() {
			assert.ConfigMapConsistentlyNotHaveKey(ctx, k8sClient, kitkyma.FluentBitSectionsConfigMap, fmt.Sprintf("%s.conf", pipelineName))
		})
	})
})
