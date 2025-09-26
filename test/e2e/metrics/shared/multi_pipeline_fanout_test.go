package shared

import (
	"io"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/metrics/runtime"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMultiPipelineFanout_Agent(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetricAgentSetB)

	var (
		uniquePrefix           = unique.Prefix()
		backendNs              = uniquePrefix("backend")
		genNs                  = uniquePrefix("gen")
		pipelineRuntimeName    = uniquePrefix("runtime")
		pipelinePrometheusName = uniquePrefix("prometheus")
	)

	backendRuntime := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName("backend-runtime"))
	backendPrometheus := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName("backend-prometheus"))
	backendRuntimeExportURL := backendRuntime.ExportURL(suite.ProxyClient)
	backendPrometheusExportURL := backendPrometheus.ExportURL(suite.ProxyClient)

	// Enable only container metrics to simplify the test setup and avoid deploying too many workloads
	// Other metric resources are tested in metrics_runtime_input_test.go, here the focus is on testing multiple pipelines withe different inputs (runtime and prometheus)
	metricPipelineRuntime := testutils.NewMetricPipelineBuilder().
		WithName(pipelineRuntimeName).
		WithRuntimeInput(true, testutils.IncludeNamespaces(genNs)).
		WithRuntimeInputContainerMetrics(true).
		WithRuntimeInputPodMetrics(false).
		WithRuntimeInputNodeMetrics(false).
		WithRuntimeInputVolumeMetrics(false).
		WithRuntimeInputDeploymentMetrics(false).
		WithRuntimeInputStatefulSetMetrics(false).
		WithRuntimeInputDaemonSetMetrics(false).
		WithRuntimeInputJobMetrics(false).
		WithOTLPOutput(testutils.OTLPEndpoint(backendRuntime.Endpoint())).
		Build()

	metricPipelinePrometheus := testutils.NewMetricPipelineBuilder().
		WithName(pipelinePrometheusName).
		WithPrometheusInput(true, testutils.IncludeNamespaces(genNs)).
		WithOTLPOutput(testutils.OTLPEndpoint(backendPrometheus.Endpoint())).
		Build()

	metricProducer := prommetricgen.New(genNs)

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&metricPipelineRuntime,
		&metricPipelinePrometheus,
		metricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		metricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
	}
	resources = append(resources, backendRuntime.K8sObjects()...)
	resources = append(resources, backendPrometheus.K8sObjects()...)

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backendRuntime)
	assert.BackendReachable(t, backendPrometheus)
	assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	assert.MetricPipelineHealthy(t, pipelineRuntimeName)
	assert.MetricPipelineHealthy(t, pipelinePrometheusName)
	assert.MetricsFromNamespaceDelivered(t, backendRuntime, genNs, runtime.DefaultMetricsNames)
	assert.MetricsFromNamespaceDelivered(t, backendPrometheus, genNs, prommetricgen.CustomMetricNames())

	Eventually(func(g Gomega) {
		resp, err := suite.ProxyClient.Get(backendRuntimeExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()

		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(bodyContent).To(HaveFlatMetrics(HaveUniqueNamesForRuntimeScope(ConsistOf(runtime.ContainerMetricsNames))), "Not all required runtime metrics are sent to runtime backend")
		checkInstrumentationScopeAndVersion(t, g, bodyContent, common.InstrumentationScopeRuntime, common.InstrumentationScopeKyma)
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).To(Succeed())

	Eventually(func(g Gomega) {
		resp, err := suite.ProxyClient.Get(backendPrometheusExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()

		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(bodyContent).To(HaveFlatMetrics(HaveUniqueNames(Not(ContainElements(runtime.DefaultMetricsNames)))), "Unwanted runtime metrics sent to prometheus backend")

		g.Expect(bodyContent).NotTo(HaveFlatMetrics(
			SatisfyAll(
				ContainElement(HaveScopeName(Equal(common.InstrumentationScopeRuntime))),
				ContainElement(HaveScopeVersion(
					SatisfyAny(
						ContainSubstring("main"),
						ContainSubstring("1."),
						ContainSubstring("PR-"),
					))),
			),
		), "scope '%v' must not be sent to the prometheus backend", common.InstrumentationScopeRuntime)
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).To(Succeed())

	Eventually(func(g Gomega) {
		resp, err := suite.ProxyClient.Get(backendPrometheusExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()

		g.Expect(err).NotTo(HaveOccurred())

		// we expect additional elements such as 'go_memstats_gc_sys_bytes'. Therefor we use 'ContainElements' instead of 'ConsistOf'
		g.Expect(bodyContent).To(HaveFlatMetrics(HaveUniqueNames(ContainElements(prommetricgen.CustomMetricNames()))), "Not all required prometheus metrics are sent to prometheus backend")

		checkInstrumentationScopeAndVersion(t, g, bodyContent, common.InstrumentationScopePrometheus, common.InstrumentationScopeKyma)
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).To(Succeed())

	// TODO: Rewrite using helper function, 2 calls:
	// - HaveUniqueNames
	// - instrumentation scope checks
	Eventually(func(g Gomega) {
		resp, err := suite.ProxyClient.Get(backendRuntimeExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()

		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(bodyContent).To(HaveFlatMetrics(HaveUniqueNames(Not(ContainElements(prommetricgen.CustomMetricNames())))), "Unwanted prometheus metrics sent to runtime backend")

		g.Expect(bodyContent).NotTo(HaveFlatMetrics(SatisfyAny(
			SatisfyAll(
				ContainElement(HaveScopeName(Equal(common.InstrumentationScopePrometheus))),
				ContainElement(HaveScopeVersion(
					SatisfyAny(
						ContainSubstring("main"),
						ContainSubstring("1."),
						ContainSubstring("PR-"),
					))),
			),
		),
		), "'%v' must not be sent to the runtime backend", common.InstrumentationScopePrometheus)
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).To(Succeed())
}

func TestMultiPipelineFanout_Gateway(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetricGatewaySetB)

	var (
		uniquePrefix  = unique.Prefix()
		backendNs     = uniquePrefix("backend")
		genNs         = uniquePrefix("gen")
		pipeline1Name = uniquePrefix("1")
		pipeline2Name = uniquePrefix("2")
	)

	backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName("backend1"))
	backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName("backend2"))

	pipeline1 := testutils.NewMetricPipelineBuilder().
		WithName(pipeline1Name).
		WithOTLPOutput(testutils.OTLPEndpoint(backend1.Endpoint())).
		Build()

	pipeline2 := testutils.NewMetricPipelineBuilder().
		WithName(pipeline2Name).
		WithOTLPOutput(testutils.OTLPEndpoint(backend2.Endpoint())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&pipeline1,
		&pipeline2,
		telemetrygen.NewPod(genNs, telemetrygen.SignalTypeMetrics).K8sObject(),
	}
	resources = append(resources, backend1.K8sObjects()...)
	resources = append(resources, backend2.K8sObjects()...)

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend1)
	assert.BackendReachable(t, backend2)
	assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	assert.MetricPipelineHealthy(t, pipeline1Name)
	assert.MetricPipelineHealthy(t, pipeline2Name)

	assert.MetricsFromNamespaceDelivered(t, backend1, genNs, telemetrygen.MetricNames)
	assert.MetricsFromNamespaceDelivered(t, backend2, genNs, telemetrygen.MetricNames)
}

// TODO:
// OPTION 1: Rewrite into checkRuntimeInputInstrumentationScope and checkPrometheusInputInstrumentationScope
// OPTION 2: HaveEach -> Contains (loosen-up test)
func checkInstrumentationScopeAndVersion(t *testing.T, g Gomega, body []byte, scope1, scope2 string) {
	t.Helper()

	g.Expect(body).To(HaveFlatMetrics(ContainElements(
		SatisfyAny(
			SatisfyAll(
				HaveScopeName(Equal(scope1)),
				HaveScopeVersion(
					SatisfyAny(
						ContainSubstring("main"),
						ContainSubstring("1."),
						ContainSubstring("PR-"),
					)),
			),
			SatisfyAll(
				HaveScopeName(Equal(scope2)),
				HaveScopeVersion(
					SatisfyAny(
						ContainSubstring("main"),
						ContainSubstring("1."),
						ContainSubstring("PR-"),
					)),
			),
			SatisfyAll(
				// the sample app is exposing some auto-instrumented metrics which the prometheus receiver will not change with the runtime scope
				HaveScopeName(Equal("go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp")),
				HaveScopeVersion(
					SatisfyAny(
						ContainSubstring("0."),
					)),
			)),
	)), "only scope '%v' or '%v' must be sent to the runtime backend", scope1, scope2)
}
