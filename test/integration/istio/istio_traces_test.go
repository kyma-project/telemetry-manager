//go:build istio

package istio

import (
	"net/http"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kittrace "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/trace"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/metricproducer"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"

	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Istio Traces", Label("tracing"), func() {
	const (
		mockNs          = "istio-tracing-mock"
		mockBackendName = "istio-tracing-backend"
		//creating mocks in a specially prepared namespace that allows calling workloads in the mesh via API server proxy
		sampleAppNs = "istio-permissive-mtls"
	)

	var (
		urls         = urlprovider.New()
		pipelineName string
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeTraces)
		objs = append(objs, mockBackend.K8sObjects()...)
		urls.SetMockBackendExport(mockBackend.Name(), mockBackend.TelemetryExportURL(proxyClient))

		istioTracePipeline := kittrace.NewPipeline("pipeline-istio-traces").WithOutputEndpointFromSecret(mockBackend.HostSecretRef())
		pipelineName = istioTracePipeline.Name()
		objs = append(objs, istioTracePipeline.K8sObject())

		traceGatewayExternalService := kitk8s.NewService("telemetry-otlp-traces-external", kitkyma.SystemNamespaceName).
			WithPort("grpc-otlp", ports.OTLPGRPC).
			WithPort("http-metrics", ports.Metrics)
		urls.SetMetrics(proxyClient.ProxyURLForService(kitkyma.SystemNamespaceName, "telemetry-otlp-traces-external", "metrics", ports.Metrics))
		objs = append(objs, traceGatewayExternalService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", "telemetry-trace-collector")))

		// Abusing metrics provider for istio traces
		sampleApp := metricproducer.New(sampleAppNs, metricproducer.WithName("trace-emitter"))
		objs = append(objs, sampleApp.Pod().K8sObject())
		urls.SetMetricPodURL(proxyClient.ProxyURLForPod(sampleAppNs, sampleApp.Name(), sampleApp.MetricsEndpoint(), sampleApp.MetricsPort()))

		return objs
	}

	Context("App with istio-sidecar", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a trace backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName, Namespace: mockNs})
		})

		It("Should have sample app running with Istio sidecar", func() {
			Eventually(func(g Gomega) {
				listOptions := client.ListOptions{
					LabelSelector: labels.SelectorFromSet(map[string]string{"app": "sample-metrics"}),
					Namespace:     sampleAppNs,
				}

				ready, err := verifiers.IsPodReady(ctx, k8sClient, listOptions)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())

				hasIstioSidecar, err := verifiers.HasContainer(ctx, k8sClient, listOptions, "istio-proxy")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(hasIstioSidecar).To(BeTrue())
			}, periodic.EventuallyTimeout*2, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should have a running trace collector deployment", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.TraceGatewayName)
		})

		It("Should have the trace pipeline running", func() {
			verifiers.TracePipelineShouldBeRunning(ctx, k8sClient, pipelineName)
		})

		It("Trace collector should answer requests", func() {
			By("Calling metrics service", func() {
				Eventually(func(g Gomega) {
					resp, err := proxyClient.Get(urls.Metrics())
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
			})
		})

		It("Should have istio-proxy spans in the backend", func() {
			By("Sending http requests", func() {
				for i := 0; i < 100; i++ {
					Eventually(func(g Gomega) {
						resp, err := proxyClient.Get(urls.MetricPodURL())
						g.Expect(err).NotTo(HaveOccurred())
						g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
					}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
				}
			})

			// Identify istio-proxy traces by component=proxy attribute
			proxyAttrs := pcommon.NewMap()
			proxyAttrs.PutStr("component", "proxy")

			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport(mockBackendName))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainSpansWithAttributes(proxyAttrs))))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should have custom spans in the backend", func() {
			// Identify sample app by serviceName attribute
			customResourceAttr := pcommon.NewMap()
			customResourceAttr.PutStr("service.name", "monitoring-custom-metrics")

			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport(mockBackendName))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainSpansWithResourceAttributes(customResourceAttr))))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
