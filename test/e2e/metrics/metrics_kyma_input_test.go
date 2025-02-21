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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelMetrics), Label(LabelSetB), Ordered, func() {
	var (
		mockNs = ID()

		pipelineWithAnnotationName   = IDWithSuffix("with-annotation")
		backendForKymaInputName      = IDWithSuffix("for-kyma-input")
		backendForKymaInputExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backendForKymaInput := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendForKymaInputName))
		objs = append(objs, backendForKymaInput.K8sObjects()...)
		backendForKymaInputExportURL = backendForKymaInput.ExportURL(ProxyClient)

		metricPipelineWithAnnotation := testutils.NewMetricPipelineBuilder().
			WithName(pipelineWithAnnotationName).
			WithOTLPOutput(testutils.OTLPEndpoint(backendForKymaInput.Endpoint())).
			Build()
		objs = append(objs, &metricPipelineWithAnnotation)

		return objs
	}

	BeforeAll(func() {
		K8sObjects := makeResources()

		DeferCleanup(func() {
			Expect(kitk8s.DeleteObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
		})

		Expect(kitk8s.CreateObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
	})

	Context("When a metricpipeline with kyma input annotation exists", Ordered, func() {

		It("Ensures the metric gateway deployment is ready", func() {
			assert.DeploymentReady(Ctx, K8sClient, kitkyma.MetricGatewayName)
		})

		It("Ensures the metrics backends are ready", func() {
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Name: backendForKymaInputName, Namespace: mockNs})
		})

		It("Ensures the metric pipelines are healthy", func() {
			assert.MetricPipelineHealthy(Ctx, K8sClient, pipelineWithAnnotationName)
		})

		It("Ensures Telemetry module status metrics are sent to the backend which is receiving metrics from the pipeline with annotation", func() {
			Eventually(func(g Gomega) {
				resp, err := ProxyClient.Get(backendForKymaInputExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				// Check the "kyma.resource.status.state" metric
				checkTelemetryModuleMetricState(g, bodyContent)

				// Check the "kyma.resource.status.conditions" metric for the "LogComponentsHealthy" condition type
				checkTelemtryModuleMetricsConditions(g, bodyContent, "LogComponentsHealthy")

				// Check the "kyma.resource.status.conditions" metric for the "MetricComponentsHealthy" condition type
				checkTelemtryModuleMetricsConditions(g, bodyContent, "MetricComponentsHealthy")

				// Check the "kyma.resource.status.conditions" metric for the "TraceComponentsHealthy" condition type
				checkTelemtryModuleMetricsConditions(g, bodyContent, "TraceComponentsHealthy")

			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Ensures metric pipeline condition metrics from both pipelines are sent to the backend which is receiving metrics from the pipeline with annotation", func() {
			Eventually(func(g Gomega) {
				resp, err := ProxyClient.Get(backendForKymaInputExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				// Check the "kyma.resource.status.conditions" type ConfigurationGenerated for  metricpipeline with annotation
				CheckMetricPipelineMetricsConditions(g, bodyContent, "ConfigurationGenerated", pipelineWithAnnotationName)

				// Check the "kyma.resource.status.conditions" type AgentHealthy for metricpipeline with annotation
				CheckMetricPipelineMetricsConditions(g, bodyContent, "AgentHealthy", pipelineWithAnnotationName)

				// Check the "kyma.resource.status.conditions" type GatewayHealthy for metricpipeline with annotation
				CheckMetricPipelineMetricsConditions(g, bodyContent, "GatewayHealthy", pipelineWithAnnotationName)

			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

	})
})

func checkTelemetryModuleMetricState(g Gomega, body []byte) {
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

func checkTelemtryModuleMetricsConditions(g Gomega, body []byte, typeName string) {
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

func CheckMetricPipelineMetricsConditions(g Gomega, body []byte, typeName, pipelineName string) {
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
