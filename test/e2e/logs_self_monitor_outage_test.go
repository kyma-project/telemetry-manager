//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"fmt"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe(suite.ID(), Label(suite.LabelSelfMonitoringLogs), Ordered, func() {
	var (
		mockNs       = suite.ID()
		pipelineName = suite.ID()
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeLogs, backend.WithReplicas(0))
		logProducer := loggen.New(mockNs).WithReplicas(2).WithLoad(loggen.LoadHigh)
		objs = append(objs, backend.K8sObjects()...)
		objs = append(objs, logProducer.K8sObject())

		logPipeline := kitk8s.NewLogPipelineV1Alpha1(pipelineName).
			WithSecretKeyRef(backend.HostSecretRefV1Alpha1()).
			WithHTTPOutput()
		objs = append(objs, logPipeline.K8sObject())

		return objs
	}

	Context("Before deploying a logpipeline", func() {
		It("Should have a healthy webhook", func() {
			verifiers.WebhookShouldBeHealthy(ctx, k8sClient)
		})
	})

	Context("When a logpipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running logpipeline", func() {
			verifiers.LogPipelineShouldBeHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have a running self-monitor", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.SelfMonitorName)
		})

		It("Should have a log backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: backend.DefaultName})
		})

		It("Should have a log producer running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: loggen.DefaultName})
		})

		It("Should have TypeFlowHealthy condition set to True", func() {
			var prevCond *metav1.Condition
			Eventually(func(g Gomega) {
				var pipeline telemetryv1alpha1.LogPipeline
				key := types.NamespacedName{Name: pipelineName}
				g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
				currCond := meta.FindStatusCondition(pipeline.Status.Conditions, conditions.TypeFlowHealthy)
				if prevCond != nil && (prevCond.Reason != currCond.Reason || prevCond.Status != currCond.Status) {
					fmt.Fprintf(GinkgoWriter, "Condition reason changed from [%s]%s to [%s]%s\n",
						prevCond.Status, prevCond.Reason, currCond.Status, currCond.Reason)
				}

				prevCond = currCond
			}, 1000*periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Not(Succeed()))
		})
	})
})
