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
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Ordered, func() {
	Context("When a metric pipeline with ONLY node metrics enabled exists", Ordered, func() {
		var (
			mockNs = suite.ID()

			backendOnlyNodeMetricsEnabledName  = suite.ID()
			pipelineOnlyNodeMetricsEnabledName = suite.ID()
			backendOnlyNodeMetricsEnabledURL   string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			backendOnlyNodeMetricsEnabled := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendOnlyNodeMetricsEnabledName))
			objs = append(objs, backendOnlyNodeMetricsEnabled.K8sObjects()...)
			backendOnlyNodeMetricsEnabledURL = backendOnlyNodeMetricsEnabled.ExportURL(proxyClient)

			pipelineOnlyNodeMetricsEnabled := testutils.NewMetricPipelineBuilder().
				WithName(pipelineOnlyNodeMetricsEnabledName).
				WithRuntimeInput(true).
				WithRuntimeInputContainerMetrics(false).
				WithRuntimeInputPodMetrics(false).
				WithRuntimeInputNodeMetrics(true).
				WithRuntimeInputVolumeMetrics(false).
				WithOTLPOutput(testutils.OTLPEndpoint(backendOnlyNodeMetricsEnabled.Endpoint())).
				Build()
			objs = append(objs, &pipelineOnlyNodeMetricsEnabled)

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
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineOnlyNodeMetricsEnabledName)
		})

		It("Ensures the metric gateway deployment is ready", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Ensures the metric agent daemonset is ready", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Should have metrics backends running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendOnlyNodeMetricsEnabledName, Namespace: mockNs})
		})

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
	})
})
