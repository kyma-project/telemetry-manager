package metrics

import (
	"io"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestKymaInput(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetricsSetA)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		backendName  = uniquePrefix("be")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName(backendName))
	pipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		&pipeline,
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	assert.DeploymentReady(t, types.NamespacedName{Name: backendName, Namespace: backendNs})
	assert.MetricPipelineHealthy(t, pipelineName)

	Eventually(func(g Gomega) {
		backendURL := backend.ExportURL(suite.ProxyClient)
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
		backendURL := backend.ExportURL(suite.ProxyClient)
		resp, err := suite.ProxyClient.Get(backendURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())

		// Check the "kyma.resource.status.conditions" type ConfigurationGenerated for  metricpipeline with annotation
		checkMetricPipelineMetricsConditions(t, g, bodyContent, "ConfigurationGenerated", pipelineName)

		// Check the "kyma.resource.status.conditions" type AgentHealthy for metricpipeline with annotation
		checkMetricPipelineMetricsConditions(t, g, bodyContent, "AgentHealthy", pipelineName)

		// Check the "kyma.resource.status.conditions" type GatewayHealthy for metricpipeline with annotation
		checkMetricPipelineMetricsConditions(t, g, bodyContent, "GatewayHealthy", pipelineName)
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
			HaveResourceAttributes(HaveKeyWithValue("k8s.resource.version", "v1alpha1")),
			HaveResourceAttributes(HaveKeyWithValue("k8s.resource.kind", "telemetries")),
			HaveScopeName(Equal(metric.InstrumentationScopeKyma)),
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
			HaveResourceAttributes(HaveKeyWithValue("k8s.resource.version", "v1alpha1")),
			HaveResourceAttributes(HaveKeyWithValue("k8s.resource.kind", "telemetries")),
			HaveScopeName(Equal(metric.InstrumentationScopeKyma)),
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
			HaveResourceAttributes(HaveKeyWithValue("k8s.resource.version", "v1alpha1")),
			HaveResourceAttributes(HaveKeyWithValue("k8s.resource.kind", "metricpipelines")),
			HaveScopeName(Equal(metric.InstrumentationScopeKyma)),
			HaveScopeVersion(SatisfyAny(
				Equal("main"),
				MatchRegexp("[0-9]+.[0-9]+.[0-9]+"),
			)),
		)),
	))
}
