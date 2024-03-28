//go:build e2e

package e2e

import (
	"fmt"
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
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics Self Monitor", Label("self-mon-metrics"), Ordered, func() {
	const (
		mockBackendName = "metrics-receiver-selfmon"
		mockNs          = "metrics-basic-selfmon-test"
	)

	var (
		pipelineName       string
		telemetryExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeMetrics)
		objs = append(objs, mockBackend.K8sObjects()...)
		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)

		pipeline := kitk8s.NewMetricPipelineV1Alpha1(fmt.Sprintf("%s-pipeline", mockBackendName)).
			WithOutputEndpointFromSecret(mockBackend.HostSecretRefV1Alpha1())
		pipelineName = pipeline.Name()
		objs = append(objs,
			telemetrygen.New(kitkyma.DefaultNamespaceName, telemetrygen.SignalTypeMetrics).K8sObject(),
			pipeline.K8sObject(),
		)

		return objs
	}

	Context("When a metric pipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running self-monitor", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.SelfMonitorName)
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

		It("Should have a metrics backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName, Namespace: mockNs})
		})

		It("Should have a running pipeline", func() {
			verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should deliver telemetrygen metrics", func() {
			verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, telemetryExportURL, kitkyma.DefaultNamespaceName, telemetrygen.MetricNames)
		})

		It("Should have TypeFlowHealthy condition set to True", func() {
			//TODO: add the conditions.TypeFlowHealthy check to verifiers.MetricPipelineShouldBeHealthy after self monitor is released
			Eventually(func(g Gomega) {
				var pipeline telemetryv1alpha1.MetricPipeline
				key := types.NamespacedName{Name: pipelineName}
				g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeFlowHealthy)).To(BeTrue())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should ensure that the self-monitor webhook has been called", func() {
			// Pushing metrics to the metric gateway triggers an alert, which in turn makes the self-monitor call the webhook
			verifiers.SelfMonitorWebhookShouldHaveBeenCalled(proxyClient)
		})
	})
})
