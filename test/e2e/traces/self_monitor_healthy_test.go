//go:build e2e

package traces

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
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelSelfMonitoringTracesHealthy), Ordered, func() {
	var (
		mockNs           = ID()
		pipelineName     = ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeTraces)
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(ProxyClient)

		tracePipeline := testutils.NewTracePipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()
		objs = append(objs,
			&tracePipeline,
			telemetrygen.NewPod(kitkyma.DefaultNamespaceName, telemetrygen.SignalTypeTraces).K8sObject(),
		)

		return objs
	}

	Context("When a trace pipeline exists", Ordered, func() {
		BeforeAll(func() {
			K8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
		})

		It("Should have a running self-monitor", func() {
			assert.DeploymentReady(Ctx, K8sClient, kitkyma.SelfMonitorName)
		})

		It("Should have a network policy deployed", func() {
			var networkPolicy networkingv1.NetworkPolicy
			Expect(K8sClient.Get(Ctx, kitkyma.SelfMonitorNetworkPolicy, &networkPolicy)).To(Succeed())

			Eventually(func(g Gomega) {
				var podList corev1.PodList
				g.Expect(K8sClient.List(Ctx, &podList, client.InNamespace(kitkyma.SystemNamespaceName), client.MatchingLabels{"app.kubernetes.io/name": kitkyma.SelfMonitorBaseName})).To(Succeed())
				g.Expect(podList.Items).NotTo(BeEmpty())

				selfMonitorPodName := podList.Items[0].Name
				pprofEndpoint := ProxyClient.ProxyURLForPod(kitkyma.SystemNamespaceName, selfMonitorPodName, "debug/pprof/", ports.Pprof)

				resp, err := ProxyClient.Get(pprofEndpoint)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusServiceUnavailable))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should have service deployed", func() {
			var service corev1.Service
			Expect(K8sClient.Get(Ctx, kitkyma.SelfMonitorName, &service)).To(Succeed())
		})

		It("Should have a running pipeline", func() {
			assert.TracePipelineHealthy(Ctx, K8sClient, pipelineName)
		})

		It("Should have a running trace gateway deployment", func() {
			assert.DeploymentReady(Ctx, K8sClient, kitkyma.TraceGatewayName)
		})

		It("Should deliver telemetrygen traces", func() {
			assert.TracesFromNamespaceDelivered(ProxyClient, backendExportURL, kitkyma.DefaultNamespaceName)
		})

		It("The telemetryFlowHealthy condition should be true", func() {
			// TODO: add the conditions.TypeFlowHealthy check to assert.TracePipelineHealthy after self monitor is released
			Eventually(func(g Gomega) {
				var pipeline telemetryv1alpha1.TracePipeline
				key := types.NamespacedName{Name: pipelineName}
				g.Expect(K8sClient.Get(Ctx, key, &pipeline)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeFlowHealthy)).To(BeTrueBecause("Flow not healthy"))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})
	})
})
