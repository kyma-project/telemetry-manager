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

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
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

var _ = Describe(suite.ID(), Label(suite.LabelMetrics, suite.LabelExperimental), Ordered, func() {
	Context("When metric pipelines runtime metrics with pod and container metrics are enabled ", Ordered, func() {
		var (
			mockNs = suite.ID()

			backendK8sClusterContainerMetricsEnabledName  = suite.IDWithSuffix("k8s-cluster-container-metrics")
			pipelineK8sClusterContainerMetricsEnabledName = suite.IDWithSuffix("k8s-cluster-container-metrics")
			backendK8sClusterContainerMetricsEnabledURL   string

			backendK8sClusterPodMetricsEnabledName  = suite.IDWithSuffix("k8s-cluster-pod-metrics")
			pipelineK8sClusterPodMetricsEnabledName = suite.IDWithSuffix("k8s-clusterpod-metrics")
			backendK8sClusterPodMetricsEnabledURL   string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			backendContainerMetricsEnabled := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendK8sClusterContainerMetricsEnabledName))
			objs = append(objs, backendContainerMetricsEnabled.K8sObjects()...)
			backendK8sClusterContainerMetricsEnabledURL = backendContainerMetricsEnabled.ExportURL(proxyClient)

			pipelineOnlyContainerMetricsEnabled := testutils.NewMetricPipelineBuilder().
				WithName(pipelineK8sClusterContainerMetricsEnabledName).
				WithRuntimeInput(true).
				WithRuntimeInputPodMetrics(false).
				WithOTLPOutput(testutils.OTLPEndpoint(backendContainerMetricsEnabled.Endpoint())).
				Build()
			objs = append(objs, &pipelineOnlyContainerMetricsEnabled)

			backendPodMetricsEnabled := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendK8sClusterPodMetricsEnabledName))
			objs = append(objs, backendPodMetricsEnabled.K8sObjects()...)
			backendK8sClusterPodMetricsEnabledURL = backendPodMetricsEnabled.ExportURL(proxyClient)

			pipelineOnlyPodMetricsEnabled := testutils.NewMetricPipelineBuilder().
				WithName(pipelineK8sClusterPodMetricsEnabledName).
				WithRuntimeInput(true).
				WithRuntimeInputContainerMetrics(false).
				WithOTLPOutput(testutils.OTLPEndpoint(backendPodMetricsEnabled.Endpoint())).
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
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineK8sClusterContainerMetricsEnabledName)
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineK8sClusterPodMetricsEnabledName)
		})

		It("Should have metrics backends running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendK8sClusterContainerMetricsEnabledName, Namespace: mockNs})
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendK8sClusterPodMetricsEnabledName, Namespace: mockNs})
		})

		It("Should deliver runtime container metrics to container-metrics backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendK8sClusterContainerMetricsEnabledURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				containerMetricNames := slices.Concat(kubeletstats.ContainerMetricsNames, k8scluster.ContainerMetricsNames)
				g.Expect(bodyContent).To(WithFlatMetrics(WithNames(
					OnlyContainElementsOf(containerMetricNames))), "Found container metrics in backend that are not part of k8scluster or kubeletstats")

				g.Expect(bodyContent).To(WithFlatMetrics(
					ContainElement(HaveField("Name", BeElementOf(k8scluster.ContainerMetricsNames)))), "Found container metrics in backend that are not part of k8scluster")

				g.Expect(bodyContent).To(WithFlatMetrics(
					ContainElement(HaveField("Name", BeElementOf(kubeletstats.ContainerMetricsNames)))), "Found container metrics in backend that are not part of kubeletstats")

				g.Expect(bodyContent).To(WithFlatMetrics(WithScopeAndVersion(ContainElement(And(
					HaveField("Name", metric.InputSourceRuntime),
					HaveField("Version",
						SatisfyAny(
							ContainSubstring("main"),
							ContainSubstring("1."),
							ContainSubstring("PR-"),
						)))))), "Only scope '%v' must be sent to the runtime backend", metric.InstrumentationScopeRuntime)

			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should deliver runtime pod metrics to pod-metrics backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendK8sClusterPodMetricsEnabledURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				podMetricNames := slices.Concat(kubeletstats.PodMetricsNames, k8scluster.PodMetricsNames)

				g.Expect(bodyContent).To(WithFlatMetrics(WithNames(
					OnlyContainElementsOf(podMetricNames))), "Found pod metrics in backend that are not part of k8scluster or kubeletstats")

				g.Expect(bodyContent).To(WithFlatMetrics(
					ContainElement(HaveField("Name", BeElementOf(k8scluster.PodMetricsNames)))), "Found pod metrics in backend that are not part of k8scluster")

				g.Expect(bodyContent).To(WithFlatMetrics(
					ContainElement(HaveField("Name", BeElementOf(kubeletstats.PodMetricsNames)))), "Found pod metrics in backend that are not part of kubeletstats")

				g.Expect(bodyContent).To(WithFlatMetrics(WithScopeAndVersion(ContainElement(And(
					HaveField("Name", metric.InputSourceRuntime),
					HaveField("Version",
						SatisfyAny(
							ContainSubstring("main"),
							ContainSubstring("1."),
							ContainSubstring("PR-"),
						)))))), "Only scope '%v' must be sent to the runtime backend", metric.InputSourceRuntime)

			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
