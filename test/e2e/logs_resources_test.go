//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogs), Ordered, func() {
	var pipelineName = suite.ID()
	const ownerReferenceKind = "LogPipeline"

	Context("When a LogPipeline exists", Ordered, func() {

		BeforeAll(func() {
			pipeline := testutils.NewLogPipelineBuilder().WithName(pipelineName).Build()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, &pipeline)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, &pipeline)).Should(Succeed())
		})

		It("Should have a DaemonSet owned by the LogPipeline", func() {
			var daemonSet appsv1.DaemonSet
			assert.HasOwnerReference(ctx, k8sClient, &daemonSet, kitkyma.FluentBitDaemonSet, ownerReferenceKind, pipelineName)
		})

		It("Should have a DaemonSet with correct pod priority class", func() {
			Eventually(func(g Gomega) {
				var daemonSet appsv1.DaemonSet
				g.Expect(k8sClient.Get(ctx, kitkyma.FluentBitDaemonSet, &daemonSet)).To(Succeed())

				g.Expect(daemonSet.Spec.Template.Spec.PriorityClassName).To(Equal("telemetry-priority-class-high"))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

	})
})
