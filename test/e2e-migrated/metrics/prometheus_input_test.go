package metrics

import (
	"io"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
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
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestPrometheusInput(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetricsSetB)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)
	metricProducer := prommetricgen.New(genNs)

	pipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithPrometheusInput(true, testutils.IncludeNamespaces(genNs)).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&pipeline,
		metricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		metricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	assert.DaemonSetReady(t, kitkyma.MetricAgentName)
	assert.MetricPipelineHealthy(t, pipelineName)

	Eventually(func(g Gomega) {
		backendURL := backend.ExportURL(suite.ProxyClient)
		resp, err := suite.ProxyClient.Get(backendURL)
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
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed(), "Annotated Pods scraping failed")

	Eventually(func(g Gomega) {
		backendURL := backend.ExportURL(suite.ProxyClient)
		resp, err := suite.ProxyClient.Get(backendURL)
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
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed(), "Annotated Services scraping failed")

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatMetrics(
			Not(ContainElement(HaveName(BeElementOf(runtime.DefaultMetricsNames)))),
		), "Unwanted runtime metrics sent to backend")

	assert.MetricsWithScopeAndNamespaceNotDelivered(t, backend,
		metric.InstrumentationScopePrometheus,
		kitkyma.SystemNamespaceName,
		"Unwanted kubeletstats metrics from system namespace sent to backend")

	t.Log("Ensures no diagnostic metrics are sent to backend")
	assert.BackendDataConsistentlyMatches(t, backend, HaveFlatMetrics(
		Not(ContainElement(HaveName(BeElementOf(diagnosticMetrics...)))),
	), "Unwanted diagnostic metrics sent to backend")
}
