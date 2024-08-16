//go:build e2e

package e2e

import (
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/otel/kubeletstats"
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
				WithRuntimeInputPodMetrics(false).
				WithOTLPOutput(testutils.OTLPEndpoint(backendOnlyContainerMetricsEnabled.Endpoint())).
				Build()
			objs = append(objs, &pipelineOnlyContainerMetricsEnabled)

			backendOnlyPodMetricsEnabled := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendOnlyPodMetricsEnabledName))
			objs = append(objs, backendOnlyPodMetricsEnabled.K8sObjects()...)
			backendOnlyPodMetricsEnabledURL = backendOnlyPodMetricsEnabled.ExportURL(proxyClient)

			pipelineOnlyPodMetricsEnabled := testutils.NewMetricPipelineBuilder().
				WithName(pipelineOnlyPodMetricsEnabledName).
				WithRuntimeInput(true).
				WithRuntimeInputContainerMetrics(false).
				WithOTLPOutput(testutils.OTLPEndpoint(backendOnlyPodMetricsEnabled.Endpoint())).
				Build()
			objs = append(objs, &pipelineOnlyPodMetricsEnabled)

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

		It("Should deliver runtime container metrics to container-metrics backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendOnlyContainerMetricsEnabledURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					WithFlatMetrics(ContainElement(HaveField("Name", BeElementOf(kubeletstats.ContainerMetricsNames)))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should not deliver runtime pod metrics to container-metrics backend", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(backendOnlyContainerMetricsEnabledURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					WithFlatMetrics(Not(ContainElement(HaveField("Name", BeElementOf(kubeletstats.PodMetricsNames))))),
				))
			}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should deliver runtime pod metrics to pod-metrics backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendOnlyPodMetricsEnabledURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					WithFlatMetrics(ContainElement(HaveField("Name", BeElementOf(kubeletstats.PodMetricsNames)))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should not deliver runtime container metrics to pod-metrics backend", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(backendOnlyPodMetricsEnabledURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					WithFlatMetrics(Not(ContainElement(HaveField("Name", BeElementOf(kubeletstats.ContainerMetricsNames))))),
				))
			}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
