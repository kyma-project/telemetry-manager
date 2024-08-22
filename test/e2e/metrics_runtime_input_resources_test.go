//go:build e2e

package e2e

import (
	"io"
	"net/http"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/otel/k8scluster"
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

		It("Should have metrics backends running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendOnlyContainerMetricsEnabledName, Namespace: mockNs})
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendOnlyPodMetricsEnabledName, Namespace: mockNs})
		})

		It("Should deliver runtime container metrics to container-metrics backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendOnlyContainerMetricsEnabledURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				containerMetricNames := slices.Concat(kubeletstats.ContainerMetricsNames, k8scluster.ContainerMetricsNames)
				g.Expect(bodyContent).To(WithFlatMetrics(WithNames(
					OnlyContainElementsOf(containerMetricNames))), "Found container metrics in backend that are not part of k8scluster or kubeletstats")

				g.Expect(bodyContent).To(WithFlatMetrics(
					ContainElement(HaveField("Name", BeElementOf(k8scluster.ContainerMetricsNames)))), "Did not find any k8scluster container metrics in backend")

				g.Expect(bodyContent).To(WithFlatMetrics(
					ContainElement(HaveField("Name", BeElementOf(kubeletstats.ContainerMetricsNames)))), "Did not find any kubeletstats container metrics in backend")

			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should not deliver runtime pod metrics to container-metrics backend", func() {
			Consistently(func(g Gomega) {
				podMetricNames := slices.Concat(kubeletstats.PodMetricsNames, k8scluster.PodMetricsNames)

				resp, err := proxyClient.Get(backendOnlyContainerMetricsEnabledURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					WithFlatMetrics(Not(ContainElement(HaveField("Name", BeElementOf(podMetricNames))))),
				))
			}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should deliver runtime pod metrics to pod-metrics backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendOnlyPodMetricsEnabledURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				podMetricNames := slices.Concat(kubeletstats.PodMetricsNames, k8scluster.PodMetricsNames)

				g.Expect(bodyContent).To(WithFlatMetrics(WithNames(
					OnlyContainElementsOf(podMetricNames))), "Found pod metrics in backend that are not part of k8scluster or kubeletstats")

				g.Expect(bodyContent).To(WithFlatMetrics(
					ContainElement(HaveField("Name", BeElementOf(k8scluster.PodMetricsNames)))), "Did not find any k8scluster pod metrics in backend")

				g.Expect(bodyContent).To(WithFlatMetrics(
					ContainElement(HaveField("Name", BeElementOf(kubeletstats.PodMetricsNames)))), "Did not find any kubeletstats pod metrics in backend")

			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should not deliver runtime container metrics to pod-metrics backend", func() {
			Consistently(func(g Gomega) {
				containerMetricNames := slices.Concat(kubeletstats.ContainerMetricsNames, k8scluster.ContainerMetricsNames)

				resp, err := proxyClient.Get(backendOnlyPodMetricsEnabledURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					WithFlatMetrics(Not(ContainElement(HaveField("Name", BeElementOf(containerMetricNames))))),
				))
			}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
