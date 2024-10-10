//go:build e2e

package e2e

import (
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/metrics/runtime"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Ordered, func() {
	Context("When a metric pipeline with ONLY pod metrics enabled exists", Ordered, func() {
		var (
			mockNs = suite.IDWithSuffix("pod-metrics")

			backendOnlyPodMetricsEnabledName  = suite.IDWithSuffix("pod-metrics")
			pipelineOnlyPodMetricsEnabledName = suite.IDWithSuffix("pod-metrics")
			backendOnlyPodMetricsEnabledURL   string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			backendOnlyPodMetricsEnabled := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendOnlyPodMetricsEnabledName))
			objs = append(objs, backendOnlyPodMetricsEnabled.K8sObjects()...)
			backendOnlyPodMetricsEnabledURL = backendOnlyPodMetricsEnabled.ExportURL(proxyClient)

			pipelineOnlyPodMetricsEnabled := testutils.NewMetricPipelineBuilder().
				WithName(pipelineOnlyPodMetricsEnabledName).
				WithRuntimeInput(true).
				WithRuntimeInputContainerMetrics(false).
				WithRuntimeInputPodMetrics(true).
				WithRuntimeInputNodeMetrics(false).
				WithRuntimeInputVolumeMetrics(false).
				WithOTLPOutput(testutils.OTLPEndpoint(backendOnlyPodMetricsEnabled.Endpoint())).
				Build()
			objs = append(objs, &pipelineOnlyPodMetricsEnabled)

			metricProducer := prommetricgen.New(mockNs)

			objs = append(objs, []client.Object{
				metricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				metricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
			}...)

			return objs
		}

		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have healthy pipelines", func() {
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineOnlyPodMetricsEnabledName)
		})

		It("Ensures the metric gateway deployment is ready", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Ensures the metric agent daemonset is ready", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Should have metrics backends running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendOnlyPodMetricsEnabledName, Namespace: mockNs})
		})

		It("Should deliver ONLY runtime pod metrics to pod-metrics backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendOnlyPodMetricsEnabledURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				g.Expect(resp).To(HaveHTTPBody(
					HaveFlatMetrics(HaveUniqueNamesForRuntimeScope(ConsistOf(runtime.PodMetricsNames))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should have expected resource attributes in runtime pod metrics", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendOnlyPodMetricsEnabledURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				g.Expect(resp).To(HaveHTTPBody(
					HaveFlatMetrics(ContainElement(HaveResourceAttributes(HaveKeys(ConsistOf(runtime.PodMetricsResourceAttributes))))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should have expected metric attributes in runtime pod metrics", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendOnlyPodMetricsEnabledURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				podNetworkErrorsMetric := "k8s.pod.network.errors"
				g.Expect(bodyContent).To(HaveFlatMetrics(
					ContainElement(SatisfyAll(
						HaveName(Equal(podNetworkErrorsMetric)),
						HaveMetricAttributes(HaveKeys(ConsistOf(runtime.PodMetricsAttributes[podNetworkErrorsMetric]))),
					)),
				))

				podNetworkIOMetric := "k8s.pod.network.io"
				g.Expect(bodyContent).To(HaveFlatMetrics(
					ContainElement(SatisfyAll(
						HaveName(Equal(podNetworkIOMetric)),
						HaveMetricAttributes(HaveKeys(ConsistOf(runtime.PodMetricsAttributes[podNetworkIOMetric]))),
					)),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
