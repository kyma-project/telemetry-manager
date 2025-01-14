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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
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
			WithPrometheusInput(true).
			WithPrometheusInputDiagnosticMetrics(true).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()
		objs = append(objs, &metricPipeline)

		return objs
	}

	Context("When a MetricPipeline with Prometheus input exists and diagnostic metrics are enabled", Ordered, func() {
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

		It("Ensures the metrics backend is ready", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Ensures the metric agent daemonset is ready", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Ensures the MetricPipeline is running", func() {
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineName)
		})

		It("Ensures all defined diagnostic metrics are sent to the backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					HaveFlatMetrics(HaveUniqueNames(ContainElements("up", "scrape_duration_seconds", "scrape_samples_scraped", "scrape_samples_post_metric_relabeling", "scrape_series_added")))))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
