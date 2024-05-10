//go:build e2e

package e2e

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
)

var _ = Describe(suite.ID(), Label(suite.LabelSelfMonitoringTraces), Ordered, func() {
	var (
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeTraces)
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(proxyClient)

		pipeline := kitk8s.NewTracePipelineV1Alpha1(pipelineName).
			WithOutputEndpointFromSecret(backend.HostSecretRefV1Alpha1())
		objs = append(objs,
			pipeline.K8sObject(),
			telemetrygen.New(kitkyma.DefaultNamespaceName, telemetrygen.SignalTypeTraces).K8sObject(),
		)

		return objs
	}

	Context("When a trace pipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running self-monitor", func() {
			assert.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.SelfMonitorName)
		})

		It("Should have a network policy deployed", func() {
			var networkPolicy networkingv1.NetworkPolicy
			Expect(k8sClient.Get(ctx, kitkyma.SelfMonitorNetworkPolicy, &networkPolicy)).To(Succeed())

			Eventually(func(g Gomega) {
				var podList corev1.PodList
				g.Expect(k8sClient.List(ctx, &podList, client.InNamespace(kitkyma.SystemNamespaceName), client.MatchingLabels{"app.kubernetes.io/name": kitkyma.SelfMonitorBaseName})).To(Succeed())
				g.Expect(podList.Items).NotTo(BeEmpty())

				selfMonitorPodName := podList.Items[0].Name
				pprofEndpoint := proxyClient.ProxyURLForPod(kitkyma.SystemNamespaceName, selfMonitorPodName, "debug/pprof/", ports.Pprof)

				resp, err := proxyClient.Get(pprofEndpoint)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusServiceUnavailable))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should have service deployed", func() {
			var service corev1.Service
			Expect(k8sClient.Get(ctx, kitkyma.SelfMonitorName, &service)).To(Succeed())
		})

		It("Should have a running pipeline", func() {
			assert.TracePipelineShouldBeHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should deliver telemetrygen traces", func() {
			assert.TracesFromNamespaceShouldBeDelivered(proxyClient, backendExportURL, kitkyma.DefaultNamespaceName)
		})

		It("The telemetryFlowHealthy condition should be true", func() {
			//TODO: add the conditions.TypeFlowHealthy check to verifiers.TracePipelineShouldBeHealthy after self monitor is released
			Eventually(func(g Gomega) {
				var pipeline telemetryv1alpha1.TracePipeline
				key := types.NamespacedName{Name: pipelineName}
				g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeFlowHealthy)).To(BeTrue())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should ensure that the self-monitor webhook has been called", func() {
			// Pushing traces to the trace gateway triggers an alert, which in turn makes the self-monitor call the webhook
			assert.SelfMonitorWebhookShouldHaveBeenCalled(proxyClient)
		})
	})
})
