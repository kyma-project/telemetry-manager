//go:build e2e

package e2e

import (
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
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeMetrics)
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(proxyClient)

		metricPipeline := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithAnnotations(map[string]string{gateway.KymaInputAnnotation: "true"}).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()
		objs = append(objs, &metricPipeline)

		return objs
	}

	Context("When a metricpipeline with kyma input annotation exists", Ordered, func() {
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

		It("Ensures the metricpipeline is healthy", func() {
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineName)
		})

		It("Ensures Telemetry module status metrics are sent to backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainMd(SatisfyAll(
						ContainResourceAttrs(HaveKeyWithValue("k8s.namespace.name", kitkyma.SystemNamespaceName)),
						ContainResourceAttrs(HaveKeyWithValue("kyma.module.name", "Telemetry")),
						ContainMetric(SatisfyAll(
							WithName(Equal("kyma.module.status.state")),
							ContainDataPointAttrs(HaveKey("state")),
						)),
						ContainScope(SatisfyAll(
							WithScopeName(ContainSubstring(metric.InstrumentationScopeKyma)),
							WithScopeVersion(
								SatisfyAny(
									Equal("main"),
									MatchRegexp("[0-9]+.[0-9]+.[0-9]+"),
								),
							),
						)),
					)),
					ContainMd(SatisfyAll(
						ContainResourceAttrs(HaveKeyWithValue("k8s.namespace.name", kitkyma.SystemNamespaceName)),
						ContainResourceAttrs(HaveKeyWithValue("kyma.module.name", "Telemetry")),
						ContainMetric(SatisfyAll(
							WithName(Equal("kyma.module.status.conditions")),
							ContainDataPointAttrs(HaveKeyWithValue("type", "LogComponentsHealthy")),
							ContainDataPointAttrs(HaveKey("status")),
							ContainDataPointAttrs(HaveKey("reason")),
						)),
						ContainScope(SatisfyAll(
							WithScopeName(ContainSubstring(metric.InstrumentationScopeKyma)),
							WithScopeVersion(
								SatisfyAny(
									Equal("main"),
									MatchRegexp("[0-9]+.[0-9]+.[0-9]+"),
								),
							),
						)),
					)),
					ContainMd(SatisfyAll(
						ContainResourceAttrs(HaveKeyWithValue("k8s.namespace.name", kitkyma.SystemNamespaceName)),
						ContainResourceAttrs(HaveKeyWithValue("kyma.module.name", "Telemetry")),
						ContainMetric(SatisfyAll(
							WithName(Equal("kyma.module.status.conditions")),
							ContainDataPointAttrs(HaveKeyWithValue("type", "TraceComponentsHealthy")),
							ContainDataPointAttrs(HaveKey("status")),
							ContainDataPointAttrs(HaveKey("reason")),
						)),
						ContainScope(SatisfyAll(
							WithScopeName(ContainSubstring(metric.InstrumentationScopeKyma)),
							WithScopeVersion(
								SatisfyAny(
									Equal("main"),
									MatchRegexp("[0-9]+.[0-9]+.[0-9]+"),
								),
							),
						)),
					)),
					ContainMd(SatisfyAll(
						ContainResourceAttrs(HaveKeyWithValue("k8s.namespace.name", kitkyma.SystemNamespaceName)),
						ContainResourceAttrs(HaveKeyWithValue("kyma.module.name", "Telemetry")),
						ContainMetric(SatisfyAll(
							WithName(Equal("kyma.module.status.conditions")),
							ContainDataPointAttrs(HaveKeyWithValue("type", "MetricComponentsHealthy")),
							ContainDataPointAttrs(HaveKey("status")),
							ContainDataPointAttrs(HaveKey("reason")),
						)),
						ContainScope(SatisfyAll(
							WithScopeName(ContainSubstring(metric.InstrumentationScopeKyma)),
							WithScopeVersion(
								SatisfyAny(
									Equal("main"),
									MatchRegexp("[0-9]+.[0-9]+.[0-9]+"),
								),
							),
						)),
					)),
				)))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
