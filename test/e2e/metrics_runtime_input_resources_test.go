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
	Context("When metric pipelines with non-default runtime input resources configuration exist", Ordered, func() {
		var (
			mockNs = suite.ID()

			backendOnlyContainerMetricsEnabledName  = suite.IDWithSuffix("container-metrics")
			pipelineOnlyContainerMetricsEnabledName = suite.IDWithSuffix("container-metrics")
			backendOnlyContainerMetricsEnabledURL   string

			backendOnlyPodMetricsEnabledName  = suite.IDWithSuffix("pod-metrics")
			pipelineOnlyPodMetricsEnabledName = suite.IDWithSuffix("pod-metrics")
			backendOnlyPodMetricsEnabledURL   string

			backendOnlyNodeMetricsEnabledName  = suite.IDWithSuffix("node-metrics")
			pipelineOnlyNodeMetricsEnabledName = suite.IDWithSuffix("node-metrics")
			backendOnlyNodeMetricsEnabledURL   string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			backendOnlyContainerMetricsEnabled := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendOnlyContainerMetricsEnabledName))
			objs = append(objs, backendOnlyContainerMetricsEnabled.K8sObjects()...)
			backendOnlyContainerMetricsEnabledURL = backendOnlyContainerMetricsEnabled.ExportURL(proxyClient)

			pipelineOnlyContainerMetricsEnabled := testutils.NewMetricPipelineBuilder().
				WithName(pipelineOnlyContainerMetricsEnabledName).
				WithRuntimeInput(true).
				WithRuntimeInputContainerMetrics(true).
				WithRuntimeInputPodMetrics(false).
				WithRuntimeInputNodeMetrics(false).
				WithOTLPOutput(testutils.OTLPEndpoint(backendOnlyContainerMetricsEnabled.Endpoint())).
				Build()
			objs = append(objs, &pipelineOnlyContainerMetricsEnabled)

			backendOnlyPodMetricsEnabled := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendOnlyPodMetricsEnabledName))
			objs = append(objs, backendOnlyPodMetricsEnabled.K8sObjects()...)
			backendOnlyPodMetricsEnabledURL = backendOnlyPodMetricsEnabled.ExportURL(proxyClient)

			pipelineOnlyPodMetricsEnabled := testutils.NewMetricPipelineBuilder().
				WithName(pipelineOnlyPodMetricsEnabledName).
				WithRuntimeInput(true).
				WithRuntimeInputPodMetrics(true).
				WithRuntimeInputContainerMetrics(false).
				WithRuntimeInputNodeMetrics(false).
				WithOTLPOutput(testutils.OTLPEndpoint(backendOnlyPodMetricsEnabled.Endpoint())).
				Build()
			objs = append(objs, &pipelineOnlyPodMetricsEnabled)

			backendOnlyNodeMetricsEnabled := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendOnlyNodeMetricsEnabledName))
			objs = append(objs, backendOnlyNodeMetricsEnabled.K8sObjects()...)
			backendOnlyNodeMetricsEnabledURL = backendOnlyNodeMetricsEnabled.ExportURL(proxyClient)

			pipelineOnlyNodeMetricsEnabled := testutils.NewMetricPipelineBuilder().
				WithName(pipelineOnlyNodeMetricsEnabledName).
				WithRuntimeInput(true).
				WithRuntimeInputNodeMetrics(true).
				WithRuntimeInputPodMetrics(false).
				WithRuntimeInputContainerMetrics(false).
				WithOTLPOutput(testutils.OTLPEndpoint(backendOnlyNodeMetricsEnabled.Endpoint())).
				Build()
			objs = append(objs, &pipelineOnlyNodeMetricsEnabled)

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
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineOnlyContainerMetricsEnabledName)
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineOnlyPodMetricsEnabledName)
		})

		It("Ensures the metric gateway deployment is ready", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Ensures the metric agent daemonset is ready", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Should have metrics backends running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendOnlyContainerMetricsEnabledName, Namespace: mockNs})
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendOnlyPodMetricsEnabledName, Namespace: mockNs})
		})

		Context("Runtime container metrics", func() {
			It("Should deliver ONLY runtime container metrics to container-metrics backend", func() {
				Eventually(func(g Gomega) {
					resp, err := proxyClient.Get(backendOnlyContainerMetricsEnabledURL)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

					g.Expect(resp).To(HaveHTTPBody(
						HaveFlatMetrics(HaveUniqueNamesForRuntimeScope(ConsistOf(runtime.ContainerMetricsNames))),
					))
				}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
			})

			It("Should have expected resource attributes in runtime container metrics", func() {
				Eventually(func(g Gomega) {
					resp, err := proxyClient.Get(backendOnlyContainerMetricsEnabledURL)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

					g.Expect(resp).To(HaveHTTPBody(
						HaveFlatMetrics(ContainElement(HaveResourceAttributes(HaveKeys(ConsistOf(runtime.ContainerMetricsResourceAttributes))))),
					))
				}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
			})
		})

		Context("Runtime pod metrics", func() {
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

		Context("Runtime node metrics", func() {
			It("Should deliver ONLY runtime node metrics to node-metrics backend", func() {
				Eventually(func(g Gomega) {
					resp, err := proxyClient.Get(backendOnlyNodeMetricsEnabledURL)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

					g.Expect(resp).To(HaveHTTPBody(
						HaveFlatMetrics(HaveUniqueNamesForRuntimeScope(ConsistOf(runtime.NodeMetricsNames))),
					))
				}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
			})

			It("Should have expected resource attributes in runtime node metrics", func() {
				Eventually(func(g Gomega) {
					resp, err := proxyClient.Get(backendOnlyNodeMetricsEnabledURL)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

					g.Expect(resp).To(HaveHTTPBody(
						HaveFlatMetrics(ContainElement(HaveResourceAttributes(HaveKeys(ConsistOf(runtime.NodeMetricsResourceAttributes))))),
					))
				}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
			})

			It("Should have expected metric attributes in runtime node metrics", func() {
				Eventually(func(g Gomega) {
					resp, err := proxyClient.Get(backendOnlyNodeMetricsEnabledURL)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

					bodyContent, err := io.ReadAll(resp.Body)
					defer resp.Body.Close()
					g.Expect(err).NotTo(HaveOccurred())

					nodeNetworkErrorsMetric := "k8s.node.network.errors"
					g.Expect(bodyContent).To(HaveFlatMetrics(
						ContainElement(SatisfyAll(
							HaveName(Equal(nodeNetworkErrorsMetric)),
							HaveMetricAttributes(HaveKeys(ConsistOf(runtime.NodeMetricsAttributes[nodeNetworkErrorsMetric]))),
						)),
					))

					nodeNetworkIOMetric := "k8s.node.network.io"
					g.Expect(bodyContent).To(HaveFlatMetrics(
						ContainElement(SatisfyAll(
							HaveName(Equal(nodeNetworkIOMetric)),
							HaveMetricAttributes(HaveKeys(ConsistOf(runtime.NodeMetricsAttributes[nodeNetworkIOMetric]))),
						)),
					))
				}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
			})
		})
	})
})
