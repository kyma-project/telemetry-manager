//go:build e2e

package metrics

import (
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/metrics/runtime"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Label(suite.LabelSetA), Ordered, func() {
	var (
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := kitbackend.New(mockNs, kitbackend.SignalTypeMetrics)
		metricProducer := prommetricgen.New(mockNs)
		objs = append(objs, backend.K8sObjects()...)
		objs = append(objs, []client.Object{
			metricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
			metricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		}...)
		backendExportURL = backend.ExportURL(suite.ProxyClient)

		metricPipeline := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithPrometheusInput(true, testutils.IncludeNamespaces(mockNs)).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()
		objs = append(objs, &metricPipeline)

		return objs
	}

	Context("When a metricpipeline with Prometheus input exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
			})

			Expect(kitk8s.CreateObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
		})

		It("Ensures the metric gateway deployment is ready", func() {
			assert.DeploymentReady(suite.Ctx, kitkyma.MetricGatewayName)
		})

		It("Ensures the metric agent daemonset is ready", func() {
			assert.DaemonSetReady(suite.Ctx, kitkyma.MetricAgentName)
		})

		It("Ensures the metrics backend is ready", func() {
			assert.DeploymentReady(suite.Ctx, types.NamespacedName{Name: kitbackend.DefaultName, Namespace: mockNs})
		})

		It("Ensures the metricpipeline is running", func() {
			assert.MetricPipelineHealthy(suite.Ctx, pipelineName)
		})

		It("Ensures custom metric scraped via annotated pods are sent to backend", func() {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				// here we are discovering the same metric-producer workload twice: once via the annotated service and once via the annotated pod
				// targets discovered via annotated pods must have no service label
				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				for _, metric := range prommetricgen.CustomMetrics() {
					g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(SatisfyAll(
						HaveName(Equal(metric.Name)),
						HaveType(Equal(metric.Type.String())),
					))))
				}

				// Verify that the URL parameter counter labels match the ones defined
				// in the prometheus.io/param_<name>:<value> annotations.
				// This ensures that the parameters were correctly processed and handled.
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(SatisfyAll(
					HaveName(Equal(prommetricgen.MetricPromhttpMetricHandlerRequestsTotal.Name)),
					HaveMetricAttributes(HaveKeyWithValue(
						prommetricgen.MetricPromhttpMetricHandlerRequestsTotalLabelKey,
						prommetricgen.ScrapingURLParamName)),
					HaveMetricAttributes(HaveKeyWithValue(
						prommetricgen.MetricPromhttpMetricHandlerRequestsTotalLabelVal,
						prommetricgen.ScrapingURLParamVal)),
				))))

			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Ensures custom metric scraped via annotated services are sent to backend", func() {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				for _, metric := range prommetricgen.CustomMetrics() {
					g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(SatisfyAll(
						HaveName(Equal(metric.Name)),
						HaveType(Equal(metric.Type.String())),
						HaveMetricAttributes(HaveKey("service")),
					))))
				}

				// Verify that the URL parameter counter labels match the ones defined
				// in the prometheus.io/param_<name>:<value> annotations.
				// This ensures that the parameters were correctly processed and handled.
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(SatisfyAll(
					HaveName(Equal(prommetricgen.MetricPromhttpMetricHandlerRequestsTotal.Name)),
					HaveMetricAttributes(HaveKeyWithValue(
						prommetricgen.MetricPromhttpMetricHandlerRequestsTotalLabelKey,
						prommetricgen.ScrapingURLParamName)),
					HaveMetricAttributes(HaveKeyWithValue(
						prommetricgen.MetricPromhttpMetricHandlerRequestsTotalLabelVal,
						prommetricgen.ScrapingURLParamVal)),
				))))

			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Ensures no runtime metrics are sent to backend", func() {
			Eventually(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(HaveFlatMetrics(
					Not(ContainElement(HaveName(BeElementOf(runtime.DefaultMetricsNames)))),
				)))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Ensures no kubeletstats metrics from system namespaces are sent to backend", func() {
			assert.MetricsWithScopeAndNamespaceNotDelivered(suite.ProxyClient, backendExportURL, metric.InstrumentationScopePrometheus, kitkyma.SystemNamespaceName)
		})

		It("Ensures no diagnostic metrics are sent to backend", func() {
			Consistently(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(HaveFlatMetrics(
					Not(ContainElement(HaveName(BeElementOf("up", "scrape_duration_seconds", "scrape_samples_scraped", "scrape_samples_post_metric_relabeling", "scrape_series_added")))),
				)))
			}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
