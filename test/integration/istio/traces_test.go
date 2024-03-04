//go:build istio

package istio

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/trace"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"net/http"
)

var _ = Describe("Traces", Label("traces"), Ordered, func() {
	const (
		mockNs          = "tracing-mock"
		mockIstiofiedNs = "istiofied-tracing-mock"
		mockBackendName = "tracing-backend"

		mockIstiofiedBackendName = "istio-tracing-backend" //creating mocks in a specially prepared namespace that allows calling workloads in the mesh via API server proxy

		istiofiedSampleAppNs   = "istio-permissive-mtls"
		istiofiedSampleAppName = "istiofied-trace-emitter"

		sampleAppNs   = "app-namespace"
		sampleAppName = "trace-emitter"
	)

	var (
		urls                                            = urlprovider.New()
		pipelineName                                    string
		istiofiedPipelineName                           string
		telemetryExportURL, telemetryIstiofiedExportURL string
		istiofiedAppURL, appURL                         string
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())
		objs = append(objs, kitk8s.NewNamespace(mockIstiofiedNs, kitk8s.WithIstioInjection()).K8sObject())
		objs = append(objs, kitk8s.NewNamespace(sampleAppNs).K8sObject())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeTraces)
		objs = append(objs, mockBackend.K8sObjects()...)
		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)

		mockIstiofiedBackend := backend.New(mockIstiofiedBackendName, mockIstiofiedNs, backend.SignalTypeTraces)
		objs = append(objs, mockIstiofiedBackend.K8sObjects()...)
		telemetryIstiofiedExportURL = mockBackend.TelemetryExportURL(proxyClient)

		istioTracePipeline := kitk8s.NewTracePipeline("istiofied-app-traces").WithOutputEndpointFromSecret(mockIstiofiedBackend.HostSecretRef())
		istiofiedPipelineName = istioTracePipeline.Name()
		objs = append(objs, istioTracePipeline.K8sObject())

		tracePipeline := kitk8s.NewTracePipeline("app-traces").WithOutputEndpointFromSecret(mockBackend.HostSecretRef())
		pipelineName = tracePipeline.Name()
		objs = append(objs, tracePipeline.K8sObject())

		traceGatewayExternalService := kitk8s.NewService("telemetry-otlp-traces-external", kitkyma.SystemNamespaceName).
			WithPort("grpc-otlp", ports.OTLPGRPC).
			WithPort("http-metrics", ports.Metrics)
		urls.SetMetrics(proxyClient.ProxyURLForService(kitkyma.SystemNamespaceName, "telemetry-otlp-traces-external", "metrics", ports.Metrics))
		objs = append(objs, traceGatewayExternalService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", "telemetry-trace-collector")))

		// Abusing metrics provider for istio traces
		istioSampleApp := prommetricgen.New(istiofiedSampleAppNs, prommetricgen.WithName(istiofiedSampleAppName))
		objs = append(objs, istioSampleApp.Pod().K8sObject())
		istiofiedAppURL = istioSampleApp.PodURL(proxyClient)

		sampleApp := prommetricgen.New(sampleAppNs, prommetricgen.WithName(sampleAppName))
		objs = append(objs, sampleApp.Pod().K8sObject())
		appURL = sampleApp.PodURL(proxyClient)

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
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockIstiofiedBackendName, Namespace: mockIstiofiedNs})
		})

		It("Should have sample app running with Istio sidecar", func() {
			verifyAppIsRunning(istiofiedSampleAppNs, map[string]string{"app": "sample-metrics"})
			verifySidecarPresent(istiofiedSampleAppNs, map[string]string{"app": "sample-metrics"})

		})

		It("Should have sample app without istio sidecar", func() {
			verifyAppIsRunning(sampleAppNs, map[string]string{"app": "sample-metrics"})
		})

		It("Should have a running trace collector deployment", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.TraceGatewayName)
		})

		It("Should have the trace pipelines running", func() {
			verifiers.TracePipelineShouldBeHealthy(ctx, k8sClient, pipelineName)
			verifiers.TracePipelineShouldBeHealthy(ctx, k8sClient, istiofiedPipelineName)
		})

		It("Trace collector with should answer requests", func() {
			By("Calling metrics service", func() {
				Eventually(func(g Gomega) {
					resp, err := proxyClient.Get(urls.Metrics())
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
			})
		})

		It("Should invoke istiofied and non-istiofied apps", func() {
			By("Sending http requests", func() {
				for _, podURLs := range []string{istiofiedAppURL, appURL} {
					for i := 0; i < 100; i++ {
						Eventually(func(g Gomega) {
							resp, err := proxyClient.Get(podURLs)
							g.Expect(err).NotTo(HaveOccurred())
							g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
						}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
					}
				}
			})
		})

		It("Should have istio traces from istiofied app namespace", func() {
			verifyIstioSpans(telemetryExportURL)
			verifyIstioSpans(telemetryIstiofiedExportURL)
		})
		It("Should have custom spans in the backend from istiofied workload", func() {
			verifyCustomIstiofiedAppSpans(telemetryExportURL)
			verifyCustomIstiofiedAppSpans(telemetryIstiofiedExportURL)
		})
		It("Should have custom spans in the backend from app-namespace", func() {
			verifyCustomAppSpans(telemetryExportURL)
			verifyCustomAppSpans(telemetryIstiofiedExportURL)
		})
	})
})

func verifySidecarPresent(namespace string, labelSelector map[string]string) {
	Eventually(func(g Gomega) {
		listOptions := client.ListOptions{
			LabelSelector: labels.SelectorFromSet(labelSelector),
			Namespace:     namespace,
		}

		hasIstioSidecar, err := verifiers.HasContainer(ctx, k8sClient, listOptions, "istio-proxy")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(hasIstioSidecar).To(BeTrue())
	}, periodic.EventuallyTimeout*2, periodic.DefaultInterval).Should(Succeed())
}

func verifyAppIsRunning(namespace string, labelSelector map[string]string) {
	Eventually(func(g Gomega) {
		listOptions := client.ListOptions{
			LabelSelector: labels.SelectorFromSet(labelSelector),
			Namespace:     namespace,
		}

		ready, err := verifiers.IsPodReady(ctx, k8sClient, listOptions)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ready).To(BeTrue())

	}, periodic.EventuallyTimeout*2, periodic.DefaultInterval).Should(Succeed())
}

func verifyIstioSpans(backendURL string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(backendURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		g.Expect(resp).To(HaveHTTPBody(ContainTd(SatisfyAll(
			// Identify istio-proxy traces by component=proxy attribute
			ContainSpan(WithSpanAttrs(HaveKeyWithValue("component", "proxy"))),
			ContainSpan(WithSpanAttrs(HaveKeyWithValue("istio.namespace", "istio-permissive-mtls"))),
		))))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func verifyCustomIstiofiedAppSpans(backendURL string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(backendURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		g.Expect(resp).To(HaveHTTPBody(ContainTd(SatisfyAll(
			// Identify sample app by serviceName attribute
			ContainResourceAttrs(HaveKeyWithValue("service.name", "monitoring-custom-metrics")),
			ContainResourceAttrs(HaveKeyWithValue("k8s.pod.name", "istiofied-trace-emitter")),
			ContainResourceAttrs(HaveKeyWithValue("k8s.namespace.name", "istio-permissive-mtls")),
		))))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func verifyCustomAppSpans(backendURL string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(backendURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(ContainTd(SatisfyAll(
			// Identify sample app by serviceName attribute
			ContainResourceAttrs(HaveKeyWithValue("service.name", "monitoring-custom-metrics")),
			ContainResourceAttrs(HaveKeyWithValue("k8s.pod.name", "trace-emitter")),
			ContainResourceAttrs(HaveKeyWithValue("k8s.namespace.name", "app-namespace")),
		))))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}
