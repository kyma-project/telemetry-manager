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
	var (
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeMetrics)
		metricProducer := prommetricgen.New(mockNs)
		objs = append(objs, backend.K8sObjects()...)
		objs = append(objs, []client.Object{
			metricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
			metricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		}...)
		backendExportURL = backend.ExportURL(proxyClient)

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
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})

			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Ensures the metric gateway deployment is ready", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Ensures the metric agent daemonset is ready", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Ensures the metrics backend is ready", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Ensures the metricpipeline is running", func() {
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineName)
		})

		It("Ensures custom metric scraped via annotated pods are sent to backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				// here we are discovering the same metric-producer workload twice: once via the annotated service and once via the annotated pod
				// targets discovered via annotated pods must have no service label
				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(SatisfyAll(
					HaveName(Equal(prommetricgen.MetricCPUTemperature.Name)),
					HaveType(Equal(prommetricgen.MetricCPUTemperature.Type.String())),
				))))

				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(SatisfyAll(
					HaveName(Equal(prommetricgen.MetricCPUEnergyHistogram.Name)),
					HaveType(Equal(prommetricgen.MetricCPUEnergyHistogram.Type.String())),
				))))

				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(SatisfyAll(
					HaveName(Equal(prommetricgen.MetricHardwareHumidity.Name)),
					HaveType(Equal(prommetricgen.MetricHardwareHumidity.Type.String())),
				))))

				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(SatisfyAll(
					HaveName(Equal(prommetricgen.MetricHardDiskErrorsTotal.Name)),
					HaveType(Equal(prommetricgen.MetricHardDiskErrorsTotal.Type.String())),
				))))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Ensures custom metric scraped via annotated services are sent to backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(SatisfyAll(
					HaveName(Equal(prommetricgen.MetricCPUTemperature.Name)),
					HaveType(Equal(prommetricgen.MetricCPUTemperature.Type.String())),
					HaveMetricAttributes(HaveKey("service")),
				))))

				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(SatisfyAll(
					HaveName(Equal(prommetricgen.MetricCPUEnergyHistogram.Name)),
					HaveType(Equal(prommetricgen.MetricCPUEnergyHistogram.Type.String())),
					HaveMetricAttributes(HaveKey("service")),
				))))

				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(SatisfyAll(
					HaveName(Equal(prommetricgen.MetricHardwareHumidity.Name)),
					HaveType(Equal(prommetricgen.MetricHardwareHumidity.Type.String())),
					HaveMetricAttributes(HaveKey("service")),
				))))

				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(SatisfyAll(
					HaveName(Equal(prommetricgen.MetricHardDiskErrorsTotal.Name)),
					HaveType(Equal(prommetricgen.MetricHardDiskErrorsTotal.Type.String())),
					HaveMetricAttributes(HaveKey("service")),
				))))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Ensures no runtime metrics are sent to backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(HaveFlatMetrics(
					Not(ContainElement(HaveName(BeElementOf(runtime.DefaultMetricsNames)))),
				)))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Ensures kubeletstats metrics from system namespaces are not sent to backend", func() {
			assert.MetricsFromNamespaceNotDelivered(proxyClient, backendExportURL, kitkyma.SystemNamespaceName)
		})

		It("Ensures no diagnostic metrics are sent to backend", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(HaveFlatMetrics(
					Not(ContainElement(HaveName(BeElementOf("up", "scrape_duration_seconds", "scrape_samples_scraped", "scrape_samples_post_metric_relabeling", "scrape_series_added")))),
				)))
			}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
