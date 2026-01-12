package gateway

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
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestKymaInput(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetricGatewaySetA)

	var (
		uniquePrefix            = unique.Prefix()
		pipelineNameKymaOnly    = uniquePrefix("kyma-only")
		pipelineNameKymaAndOtlp = uniquePrefix("kyma-and-otlp")
		backendNs               = uniquePrefix("backend")
		generatorNs             = uniquePrefix("generator")
	)

	backendKymaOnly := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName(pipelineNameKymaOnly))
	backendKymaAndOtlp := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName(pipelineNameKymaAndOtlp))

	// one pipeline with only Kyma input. Here we do not expect metrics from generatorNs
	pipelineWithKymaOnly := testutils.NewMetricPipelineBuilder().
		WithName(pipelineNameKymaOnly).
		WithOTLPInput(false).
		WithOTLPOutput(testutils.OTLPEndpoint(backendKymaOnly.EndpointHTTP())).
		Build()

	// one pipeline with Kyma input and additional namespace included. Here we expect metrics from generatorNs
	pipelineWithKymaAndOtlp := testutils.NewMetricPipelineBuilder().
		WithName(pipelineNameKymaAndOtlp).
		WithOTLPInput(true, testutils.IncludeNamespaces(generatorNs)).
		WithOTLPOutput(testutils.OTLPEndpoint(backendKymaAndOtlp.EndpointHTTP())).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(generatorNs).K8sObject(),
		&pipelineWithKymaOnly,
		&pipelineWithKymaAndOtlp,
		telemetrygen.NewPod(generatorNs, telemetrygen.SignalTypeMetrics).K8sObject(),
	}
	resources = append(resources, backendKymaOnly.K8sObjects()...)
	resources = append(resources, backendKymaAndOtlp.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backendKymaOnly)
	assert.BackendReachable(t, backendKymaAndOtlp)
	assert.DeploymentReady(t, kitkyma.MetricGatewayName)

	if suite.DebugObjectsEnabled() {
		objects := []client.Object{
			&pipelineWithKymaOnly,
			kitk8sobjects.NewConfigMap(kitkyma.MetricGatewayBaseName, kitkyma.SystemNamespaceName).K8sObject(),
		}
		Expect(kitk8s.ObjectsToFile(t, objects...)).To(Succeed())
	}

	assert.MetricPipelineHealthy(t, pipelineNameKymaOnly)
	assert.MetricPipelineHealthy(t, pipelineNameKymaAndOtlp)

	// Verify that metrics are delivered to both backends
	assert.MetricsFromNamespaceDelivered(t, backendKymaAndOtlp, kitkyma.SystemNamespaceName, []string{"kyma.resource.status.state"})
	assert.MetricsFromNamespaceDelivered(t, backendKymaOnly, kitkyma.SystemNamespaceName, []string{"kyma.resource.status.state"})

	// Verify that namespace specific metrics are only delivered to the kyma-and-otlp backend
	assert.MetricsFromNamespaceDelivered(t, backendKymaAndOtlp, generatorNs, telemetrygen.MetricNames)
	assert.MetricsFromNamespaceNotDelivered(t, backendKymaOnly, generatorNs)

	Eventually(func(g Gomega) {
		backendURL := backendKymaOnly.ExportURL(suite.ProxyClient)
		resp, err := suite.ProxyClient.Get(backendURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()

		g.Expect(err).NotTo(HaveOccurred())

		// Check the "kyma.resource.status.state" metric
		checkTelemetryModuleMetricState(t, g, bodyContent)

		// Check the "kyma.resource.status.conditions" metric for the "LogComponentsHealthy" condition type
		checkTelemtryModuleMetricsConditions(t, g, bodyContent, "LogComponentsHealthy")

		// Check the "kyma.resource.status.conditions" metric for the "MetricComponentsHealthy" condition type
		checkTelemtryModuleMetricsConditions(t, g, bodyContent, "MetricComponentsHealthy")

		// Check the "kyma.resource.status.conditions" metric for the "TraceComponentsHealthy" condition type
		checkTelemtryModuleMetricsConditions(t, g, bodyContent, "TraceComponentsHealthy")
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval,
		"Missing telemetry module status metrics").To(Succeed())

	Eventually(func(g Gomega) {
		backendURL := backendKymaOnly.ExportURL(suite.ProxyClient)
		resp, err := suite.ProxyClient.Get(backendURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()

		g.Expect(err).NotTo(HaveOccurred())

		// Check the "kyma.resource.status.conditions" type ConfigurationGenerated for  metricpipeline with annotation
		checkMetricPipelineMetricsConditions(t, g, bodyContent, "ConfigurationGenerated", pipelineNameKymaOnly)

		// Check the "kyma.resource.status.conditions" type AgentHealthy for metricpipeline with annotation
		checkMetricPipelineMetricsConditions(t, g, bodyContent, "AgentHealthy", pipelineNameKymaOnly)

		// Check the "kyma.resource.status.conditions" type GatewayHealthy for metricpipeline with annotation
		checkMetricPipelineMetricsConditions(t, g, bodyContent, "GatewayHealthy", pipelineNameKymaOnly)
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval,
		"Missing condition metrics").To(Succeed())
}

func checkTelemetryModuleMetricState(t *testing.T, g Gomega, body []byte) {
	t.Helper()

	g.Expect(body).To(HaveFlatMetrics(
		ContainElement(SatisfyAll(
			HaveName(Equal("kyma.resource.status.state")),
			HaveMetricAttributes(HaveKey("state")),
			HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", kitkyma.SystemNamespaceName)),
			HaveResourceAttributes(HaveKeyWithValue("k8s.resource.name", "default")),
			HaveResourceAttributes(HaveKeyWithValue("k8s.resource.group", "operator.kyma-project.io")),
			HaveResourceAttributes(HaveKeyWithValue("k8s.resource.version", "v1beta1")),
			HaveResourceAttributes(HaveKeyWithValue("k8s.resource.kind", "telemetries")),
			HaveScopeName(Equal(common.InstrumentationScopeKyma)),
			HaveScopeVersion(SatisfyAny(
				Equal("main"),
				MatchRegexp("[0-9]+.[0-9]+.[0-9]+"),
			)),
		)),
	))
}

func checkTelemtryModuleMetricsConditions(t *testing.T, g Gomega, body []byte, typeName string) {
	t.Helper()

	g.Expect(body).To(HaveFlatMetrics(
		ContainElement(SatisfyAll(
			HaveName(Equal("kyma.resource.status.conditions")),
			HaveMetricAttributes(HaveKeyWithValue("type", typeName)),
			HaveMetricAttributes(HaveKey("status")),
			HaveMetricAttributes(HaveKey("reason")),
			HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", kitkyma.SystemNamespaceName)),
			HaveResourceAttributes(HaveKeyWithValue("k8s.resource.name", "default")),
			HaveResourceAttributes(HaveKeyWithValue("k8s.resource.group", "operator.kyma-project.io")),
			HaveResourceAttributes(HaveKeyWithValue("k8s.resource.version", "v1beta1")),
			HaveResourceAttributes(HaveKeyWithValue("k8s.resource.kind", "telemetries")),
			HaveScopeName(Equal(common.InstrumentationScopeKyma)),
			HaveScopeVersion(SatisfyAny(
				Equal("main"),
				MatchRegexp("[0-9]+.[0-9]+.[0-9]+"),
			)),
		)),
	))
}

func checkMetricPipelineMetricsConditions(t *testing.T, g Gomega, body []byte, typeName, pipelineName string) {
	t.Helper()

	g.Expect(body).To(HaveFlatMetrics(
		ContainElement(SatisfyAll(
			HaveName(Equal("kyma.resource.status.conditions")),
			HaveMetricAttributes(HaveKeyWithValue("type", typeName)),
			HaveMetricAttributes(HaveKey("status")),
			HaveMetricAttributes(HaveKey("reason")),
			HaveResourceAttributes(HaveKeyWithValue("k8s.resource.name", pipelineName)),
			HaveResourceAttributes(HaveKeyWithValue("k8s.resource.group", "telemetry.kyma-project.io")),
			HaveResourceAttributes(HaveKeyWithValue("k8s.resource.version", "v1beta1")),
			HaveResourceAttributes(HaveKeyWithValue("k8s.resource.kind", "metricpipelines")),
			HaveScopeName(Equal(common.InstrumentationScopeKyma)),
			HaveScopeVersion(SatisfyAny(
				Equal("main"),
				MatchRegexp("[0-9]+.[0-9]+.[0-9]+"),
			)),
		)),
	))
}
