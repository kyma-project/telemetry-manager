//go:build e2e

package e2e

import (
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
	Context("When a metric pipeline with ONLY container metrics enabled exists", Ordered, func() {
		var (
			mockNs = suite.ID()

			backendOnlyContainerMetricsEnabledName  = suite.ID()
			pipelineOnlyContainerMetricsEnabledName = suite.ID()
			backendOnlyContainerMetricsEnabledURL   string
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
				WithRuntimeInputVolumeMetrics(false).
				WithOTLPOutput(testutils.OTLPEndpoint(backendOnlyContainerMetricsEnabled.Endpoint())).
				Build()
			objs = append(objs, &pipelineOnlyContainerMetricsEnabled)

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
		})

		It("Ensures the metric gateway deployment is ready", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Ensures the metric agent daemonset is ready", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Should have metrics backends running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendOnlyContainerMetricsEnabledName, Namespace: mockNs})
		})

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
})
