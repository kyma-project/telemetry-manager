//go:build e2e

package e2e

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/otel/kubeletstats"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe(suite.ID(), Label("mmtest"), Ordered, func() {
	Context("When multiple metric pipelines with instrumentation scope exist", Ordered, func() {
		var (
			mockNs                  = suite.ID()
			backendRuntimeName      = suite.IDWithSuffix("backend-runtime")
			pipelineRuntimeName     = suite.IDWithSuffix("runtime")
			backendRuntimeExportURL string
			backendAppName          = suite.IDWithSuffix("backend-app")
			pipelineAppName         = suite.IDWithSuffix("app")
			backendAppExportURL     string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			backendRuntime := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendRuntimeName))
			objs = append(objs, backendRuntime.K8sObjects()...)
			backendRuntimeExportURL = backendRuntime.ExportURL(proxyClient)

			metricPipelineRuntime := kitk8s.NewMetricPipelineV1Alpha1(pipelineRuntimeName).
				WithOutputEndpointFromSecret(backendRuntime.HostSecretRefV1Alpha1()).
				RuntimeInput(true)
			objs = append(objs, metricPipelineRuntime.K8sObject())

			backendApp := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendAppName))
			objs = append(objs, backendApp.K8sObjects()...)
			backendAppExportURL = backendApp.ExportURL(proxyClient)

			metricPipelineApp := kitk8s.NewMetricPipelineV1Alpha1(pipelineAppName).
				WithOutputEndpointFromSecret(backendApp.HostSecretRefV1Alpha1()).
				PrometheusInput(true)
			objs = append(objs, metricPipelineApp.K8sObject())

			metricProducer := prommetricgen.New(mockNs)

			objs = append(objs, []client.Object{
				metricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				metricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
			}...)
			return objs
		}

		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, pipelineRuntimeName)
			verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, pipelineAppName)
		})

		It("Should have a running metric gateway deployment", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Should have a metrics backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: backendRuntimeName, Namespace: mockNs})
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: backendAppName, Namespace: mockNs})
		})

		It("Ensures kubeletstats metrics are sent to runtime backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendRuntimeExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(ContainMd(SatisfyAll(
					ContainMetric(WithName(BeElementOf(kubeletstats.MetricNames))),
					WithScope(ContainElement(WithScopeName(ContainSubstring(InstrumentationScopeRuntime)))),
				))))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Ensures kubeletstats metrics are not sent to app backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendAppExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(ContainMd(Not(SatisfyAll(
					ContainMetric(WithName(BeElementOf(kubeletstats.MetricNames))),
					WithScope(ContainElement(WithScopeName(ContainSubstring(InstrumentationScopeRuntime)))),
				)))))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Ensures prometheus metrics are sent to app backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendAppExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(ContainMd(SatisfyAll(
					ContainMetric(WithName(BeElementOf(prommetricgen.MetricNames))),
					WithScope(ContainElement(WithScopeName(ContainSubstring(InstrumentationScopePrometheus)))),
				))))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Ensures prometheus metrics are not sent to runtime backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendRuntimeExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(ContainMd(Not(SatisfyAll(
					ContainMetric(WithName(BeElementOf(prommetricgen.MetricNames))),
					WithScope(ContainElement(WithScopeName(ContainSubstring(InstrumentationScopePrometheus)))),
				)))))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

	})

})
