//go:build e2e

package e2e

import (
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/gateway"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics, suite.LabelExperimental), Ordered, func() {
	var (
		mockNs = suite.ID()

		pipelineWithAnnotationName   = suite.IDWithSuffix("with-annotation")
		backendForKymaInputName      = suite.IDWithSuffix("for-kyma-input")
		backendForKymaInputExportURL string

		pipelineWithoutAnnotationName  = suite.IDWithSuffix("without-annotation")
		backendForNoKymaInputName      = suite.IDWithSuffix("for-no-kyma-input")
		backendForNoKymaInputExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backendForKymaInput := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendForKymaInputName))
		objs = append(objs, backendForKymaInput.K8sObjects()...)
		backendForKymaInputExportURL = backendForKymaInput.ExportURL(proxyClient)

		backendForNoKymaInput := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendForNoKymaInputName))
		objs = append(objs, backendForNoKymaInput.K8sObjects()...)
		backendForNoKymaInputExportURL = backendForNoKymaInput.ExportURL(proxyClient)

		metricPipelineWithAnnotation := testutils.NewMetricPipelineBuilder().
			WithName(pipelineWithAnnotationName).
			WithAnnotations(map[string]string{gateway.KymaInputAnnotation: "true"}).
			WithOTLPOutput(testutils.OTLPEndpoint(backendForKymaInput.Endpoint())).
			Build()
		objs = append(objs, &metricPipelineWithAnnotation)

		metricPipelineWithoutAnnotation := testutils.NewMetricPipelineBuilder().
			WithName(pipelineWithoutAnnotationName).
			WithOTLPOutput(testutils.OTLPEndpoint(backendForNoKymaInput.Endpoint())).
			Build()
		objs = append(objs, &metricPipelineWithoutAnnotation)

		return objs
	}

	BeforeAll(func() {
		k8sObjects := makeResources()

		DeferCleanup(func() {
			Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
	})

	Context("When a metricpipeline with kyma input annotation exists", Ordered, func() {

		It("Ensures the metric gateway deployment is ready", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Ensures the metrics backends are ready", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendForKymaInputName, Namespace: mockNs})
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backendForNoKymaInputName, Namespace: mockNs})
		})

		It("Ensures the metric pipelines are healthy", func() {
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineWithAnnotationName)
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineWithoutAnnotationName)
		})

		It("Ensures Telemetry module status metrics are sent to the backend which is receiving metrics from the pipeline with annotation", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendForKymaInputExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				// Check the "kyma.module.status.state" metric
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveName(Equal("kyma.module.status.state")))), "metric name should be 'kyma.module.status.state'")
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveMetricAttributes(HaveKey("state")))), "metric attributes should have key 'state'")
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", kitkyma.SystemNamespaceName)))), "resource attributes should have key 'k8s.namespace.name' with value '%v'", kitkyma.SystemNamespaceName)
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveResourceAttributes(HaveKeyWithValue("kyma.module.name", "Telemetry")))), "resource attributes should have key 'kyma.module.name' with value 'Telemetry'")
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveScopeName(Equal(metric.InstrumentationScopeKyma)))), "scope name should be '%v'", metric.InstrumentationScopeKyma)
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveScopeVersion(SatisfyAny(
					Equal("main"),
					MatchRegexp("[0-9]+.[0-9]+.[0-9]+"),
				)))), "scope version should be either 'main' or match the regex '[0-9]+.[0-9]+.[0-9]+'")

				// Check the "kyma.module.status.conditions" metric for the "LogComponentsHealthy" condition type
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveName(Equal("kyma.module.status.conditions")))), "metric name should be 'kyma.module.status.conditions'")
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveMetricAttributes(HaveKey("status")))), "metric attributes should have key 'status'")
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveMetricAttributes(HaveKey("reason")))), "metric attributes should have key 'reason'")
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveMetricAttributes(HaveKeyWithValue("type", "LogComponentsHealthy")))), "metric attributes should have key 'type' with value 'LogComponentsHealthy'")
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", kitkyma.SystemNamespaceName)))), "resource attributes should have key 'k8s.namespace.name' with value '%v'", kitkyma.SystemNamespaceName)
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveResourceAttributes(HaveKeyWithValue("kyma.module.name", "Telemetry")))), "resource attributes should have key 'kyma.module.name' with value 'Telemetry'")
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveScopeName(Equal(metric.InstrumentationScopeKyma)))), "scope name should be '%v'", metric.InstrumentationScopeKyma)
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveScopeVersion(SatisfyAny(
					Equal("main"),
					MatchRegexp("[0-9]+.[0-9]+.[0-9]+"),
				)))), "scope version should be either 'main' or match the regex '[0-9]+.[0-9]+.[0-9]+'")

				// Check the "kyma.module.status.conditions" metric for the "MetricComponentsHealthy" condition type
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveName(Equal("kyma.module.status.conditions")))), "metric name should be 'kyma.module.status.conditions'")
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveMetricAttributes(HaveKey("status")))), "metric attributes should have key 'status'")
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveMetricAttributes(HaveKey("reason")))), "metric attributes should have key 'reason'")
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveMetricAttributes(HaveKeyWithValue("type", "MetricComponentsHealthy")))), "metric attributes should have key 'type' with value 'MetricComponentsHealthy'")
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", kitkyma.SystemNamespaceName)))), "resource attributes should have key 'k8s.namespace.name' with value '%v'", kitkyma.SystemNamespaceName)
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveResourceAttributes(HaveKeyWithValue("kyma.module.name", "Telemetry")))), "resource attributes should have key 'kyma.module.name' with value 'Telemetry'")
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveScopeName(Equal(metric.InstrumentationScopeKyma)))), "scope name should be '%v'", metric.InstrumentationScopeKyma)
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveScopeVersion(SatisfyAny(
					Equal("main"),
					MatchRegexp("[0-9]+.[0-9]+.[0-9]+"),
				)))), "scope version should be either 'main' or match the regex '[0-9]+.[0-9]+.[0-9]+'")

				// Check the "kyma.module.status.conditions" metric for the "TraceComponentsHealthy" condition type
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveName(Equal("kyma.module.status.conditions")))), "metric name should be 'kyma.module.status.conditions'")
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveMetricAttributes(HaveKey("status")))), "metric attributes should have key 'status'")
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveMetricAttributes(HaveKey("reason")))), "metric attributes should have key 'reason'")
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveMetricAttributes(HaveKeyWithValue("type", "TraceComponentsHealthy")))), "metric attributes should have key 'type' with value 'TraceComponentsHealthy'")
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", kitkyma.SystemNamespaceName)))), "resource attributes should have key 'k8s.namespace.name' with value '%v'", kitkyma.SystemNamespaceName)
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveResourceAttributes(HaveKeyWithValue("kyma.module.name", "Telemetry")))), "resource attributes should have key 'kyma.module.name' with value 'Telemetry'")
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveScopeName(Equal(metric.InstrumentationScopeKyma)))), "scope name should be '%v'", metric.InstrumentationScopeKyma)
				g.Expect(bodyContent).To(HaveFlatMetrics(ContainElement(HaveScopeVersion(SatisfyAny(
					Equal("main"),
					MatchRegexp("[0-9]+.[0-9]+.[0-9]+"),
				)))), "scope version should be either 'main' or match the regex '[0-9]+.[0-9]+.[0-9]+'")
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Ensures Telemetry module status metrics are not sent to the backend which is receiving metrics from the pipeline without annotation", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(backendForNoKymaInputExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					HaveFlatMetrics(SatisfyAll(
						Not(ContainElement(HaveName(Equal("kyma.module.status.state")))),
						Not(ContainElement(HaveName(Equal("kyma.module.status.conditions")))),
					)),
				))
			}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
