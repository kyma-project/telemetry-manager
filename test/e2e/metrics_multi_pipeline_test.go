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

	. "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/otel/k8scluster"
	"github.com/kyma-project/telemetry-manager/test/testkit/otel/kubeletstats"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Ordered, func() {
	Context("When multiple metric pipelines with instrumentation scope exist", Ordered, func() {
		var (
			mockNs                     = suite.ID()
			backendRuntimeName         = suite.IDWithSuffix("backend-runtime")
			pipelineRuntimeName        = suite.IDWithSuffix("runtime")
			backendRuntimeExportURL    string
			backendPrometheusName      = suite.IDWithSuffix("backend-prometheus")
			pipelinePrometheusName     = suite.IDWithSuffix("prometheus")
			backendPrometheusExportURL string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			backendRuntime := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendRuntimeName))
			objs = append(objs, backendRuntime.K8sObjects()...)
			backendRuntimeExportURL = backendRuntime.ExportURL(proxyClient)

			metricPipelineRuntime := testutils.NewMetricPipelineBuilder().
				WithName(pipelineRuntimeName).
				WithRuntimeInput(true).
				WithOTLPOutput(testutils.OTLPEndpoint(backendRuntime.Endpoint())).
				Build()
			objs = append(objs, &metricPipelineRuntime)

			backendPrometheus := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendPrometheusName))
			objs = append(objs, backendPrometheus.K8sObjects()...)
			backendPrometheusExportURL = backendPrometheus.ExportURL(proxyClient)

			metricPipelinePrometheus := testutils.NewMetricPipelineBuilder().
				WithName(pipelinePrometheusName).
				WithPrometheusInput(true).
				WithOTLPOutput(testutils.OTLPEndpoint(backendPrometheus.Endpoint())).
				Build()
			objs = append(objs, &metricPipelinePrometheus)

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

		It("Should have running pipelines", func() {
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineRuntimeName)
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelinePrometheusName)
		})

		It("Should have a running metric gateway deployment", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Should have a metrics backend running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendRuntimeName, Namespace: mockNs})
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendPrometheusName, Namespace: mockNs})
		})

		It("Ensures kubeletstats metrics are sent to runtime backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendRuntimeExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				bodyContent, err := io.ReadAll(resp.Body)
				g.Expect(err).NotTo(HaveOccurred())

				expectedMetrics := slices.Concat(kubeletstats.DefaultMetricsNames, k8scluster.DefaultMetricsNames)
				g.Expect(bodyContent).To(HaveFlatMetrics(HaveUniqueNames(ConsistOf(expectedMetrics))), "Not all required kubeletstats metrics are sent to runtime backend")

				g.Expect(bodyContent).To(HaveFlatMetrics(HaveEach(HaveScopeName(Equal(InstrumentationScopeRuntime)))), "only scope name %v may exist in the runtime backend", InstrumentationScopeRuntime)
				g.Expect(bodyContent).To(HaveFlatMetrics(HaveEach(
					SatisfyAll(
						HaveScopeName(Equal(InstrumentationScopeRuntime)),
						HaveScopeVersion(
							SatisfyAny(
								ContainSubstring("main"),
								ContainSubstring("1."),
								ContainSubstring("PR-"),
							)),
					)),
				), "only scope '%v' must be sent to the runtime backend", InstrumentationScopeRuntime)
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Ensures kubeletstats metrics are not sent to prometheus backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendPrometheusExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				expectedMetrics := slices.Concat(kubeletstats.DefaultMetricsNames, k8scluster.DefaultMetricsNames)
				g.Expect(bodyContent).To(HaveFlatMetrics(HaveUniqueNames(Not(ContainElements(expectedMetrics)))), "No kubeletstats metrics must be sent to prometheus backend")

				g.Expect(bodyContent).NotTo(HaveFlatMetrics(
					SatisfyAll(
						ContainElement(HaveScopeName(Equal(InstrumentationScopeRuntime))),
						ContainElement(HaveScopeVersion(
							SatisfyAny(
								ContainSubstring("main"),
								ContainSubstring("1."),
								ContainSubstring("PR-"),
							))),
					),
				), "scope '%v' must not be sent to the prometheus backend", InstrumentationScopeRuntime)
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Ensures prometheus metrics are sent to prometheus backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendPrometheusExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				// we expect additional elements such as 'go_memstats_gc_sys_bytes'. Therefor we use 'ContainElements' instead of 'ConsistOf'
				g.Expect(bodyContent).To(HaveFlatMetrics(HaveUniqueNames(ContainElements(prommetricgen.DefaultMetricsNames))), "Not all required prometheus metrics are sent to prometheus backend")

				g.Expect(bodyContent).To(HaveFlatMetrics(HaveEach(
					SatisfyAll(
						HaveScopeName(Equal(InstrumentationScopePrometheus)),
						HaveScopeVersion(
							SatisfyAny(
								ContainSubstring("main"),
								ContainSubstring("1."),
								ContainSubstring("PR-"),
							)),
					)),
				), "Only scope '%v' must be sent to the prometheus backend", InstrumentationScopePrometheus)
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Ensures prometheus metrics are not sent to runtime backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendRuntimeExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(bodyContent).To(HaveFlatMetrics(HaveUniqueNames(Not(ContainElements(prommetricgen.DefaultMetricsNames)))), "No prometheus metrics must be sent to runtime backend")

				g.Expect(bodyContent).NotTo(HaveFlatMetrics(
					SatisfyAll(
						ContainElement(HaveScopeName(Equal(InstrumentationScopePrometheus))),
						ContainElement(HaveScopeVersion(
							SatisfyAny(
								ContainSubstring("main"),
								ContainSubstring("1."),
								ContainSubstring("PR-"),
							))),
					),
				), "'%v' must not be sent to the runtime backend", InstrumentationScopePrometheus)
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
