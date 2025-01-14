//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogs), Ordered, func() {
	var (
		mockNs       = suite.ID()
		pipelineName = suite.ID()
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		logPipeline := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithApplicationInputDisabled().
			WithHTTPOutput(
				testutils.HTTPHost("localhost"),
				testutils.HTTPPort(443),
			).
			Build()

		objs = append(objs, &logPipeline)

		return objs
	}

	Context("Before deploying a logpipeline", func() {
		It("Should have a healthy webhook", func() {
			assert.WebhookHealthy(ctx, k8sClient)
		})
	})

	Context("When a logpipeline with disabled application input exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Pipeline should have unhealthy agent condition", func() {
			assert.LogPipelineHasCondition(ctx, k8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeAgentHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonAgentNotReady,
			})
		})

		It("Fluent Bit should not be deployed", func() {
			Consistently(func(g Gomega) {
				var daemonSet appsv1.DaemonSet
				err := k8sClient.Get(ctx, kitkyma.FluentBitDaemonSetName, &daemonSet)
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed())
		})
	})
})
