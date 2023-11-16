//go:build istio

package istio

import (
	"fmt"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/trace"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"net/http"
)

var _ = Describe("Istio Traces", Label("tracing"), Ordered, func() {
	const (
		mockNs          = "tracing-mock"
		mockIstiofiedNS = "istiofied-tracing-mock"
		mockBackendName = "tracing-backend"

		mockIstiofiedBackendName = "istio-tracing-backend" //creating mocks in a specially prepared namespace that allows calling workloads in the mesh via API server proxy

		istiofiedSampleAppNs   = "istio-permissive-mtls"
		istiofiedSampleAppName = "istiofied-trace-emitter"

		sampleAppNs   = "app-namespace"
		sampleAppName = "trace-emitter"
	)

	var (
		urls = urlprovider.New()
		//pipelineName          string
		//istiofiedPipelineName string
	)

	//makeResources := func() []client.Object {
	//	var objs []client.Object
	//
	//	objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())
	//	objs = append(objs, kitk8s.NewNamespace(mockIstiofiedNS, kitk8s.WithIstioInjection()).K8sObject())
	//	objs = append(objs, kitk8s.NewNamespace(sampleAppNs).K8sObject())
	//
	//mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeTraces)
	//	objs = append(objs, mockBackend.K8sObjects()...)
	//urls.SetMockBackendExport(mockBackend.Name(), mockBackend.TelemetryExportURL(proxyClient))
	//
	//	mockIstiofiedBackend := backend.New(mockIstiofiedBackendName, mockIstiofiedNS, backend.SignalTypeTraces)
	//	objs = append(objs, mockIstiofiedBackend.K8sObjects()...)
	//	urls.SetMockBackendExport(mockIstiofiedBackend.Name(), mockIstiofiedBackend.TelemetryExportURL(proxyClient))
	//
	//	istioTracePipeline := kittrace.NewPipeline("istiofied-app-traces").WithOutputEndpointFromSecret(mockIstiofiedBackend.HostSecretRef())
	//	istiofiedPipelineName = istioTracePipeline.Name()
	//	objs = append(objs, istioTracePipeline.K8sObject())
	//
	//	tracePipeline := kittrace.NewPipeline("app-traces").WithOutputEndpointFromSecret(mockBackend.HostSecretRef())
	//	pipelineName = tracePipeline.Name()
	//	objs = append(objs, tracePipeline.K8sObject())
	//
	//	traceGatewayExternalService := kitk8s.NewService("telemetry-otlp-traces-external", kitkyma.SystemNamespaceName).
	//		WithPort("grpc-otlp", ports.OTLPGRPC).
	//		WithPort("http-metrics", ports.Metrics)
	//	urls.SetMetrics(proxyClient.ProxyURLForService(kitkyma.SystemNamespaceName, "telemetry-otlp-traces-external", "metrics", ports.Metrics))
	//	objs = append(objs, traceGatewayExternalService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", "telemetry-trace-collector")))
	//
	//	// Abusing metrics provider for istio traces
	//	istioSampleApp := metricproducer.New(istiofiedSampleAppNs, metricproducer.WithName(istiofiedSampleAppName))
	//	objs = append(objs, istioSampleApp.Pod().K8sObject())
	//	//urls.SetMetricPodURL(proxyClient.ProxyURLForPod(istiofiedSampleAppNs, istioSampleApp.Name(), istioSampleApp.MetricsEndpoint(), istioSampleApp.MetricsPort()))
	//	urls.SetMultipleMetricPodURL(istioSampleApp.Name(), proxyClient.ProxyURLForPod(istiofiedSampleAppNs, istioSampleApp.Name(), istioSampleApp.MetricsEndpoint(), istioSampleApp.MetricsPort()))
	//
	//	sampleApp := metricproducer.New(sampleAppNs, metricproducer.WithName(sampleAppName))
	//	objs = append(objs, sampleApp.Pod().K8sObject())
	//	//fmt.Printf("APP:  %+v\n", sampleApp.Pod().K8sObject())
	//	//urls.SetMetricPodURL(proxyClient.ProxyURLForPod(sampleAppNs, sampleApp.Name(), sampleApp.MetricsEndpoint(), sampleApp.MetricsPort()))
	//	urls.SetMultipleMetricPodURL(sampleApp.Name(), proxyClient.ProxyURLForPod(sampleAppNs, sampleApp.Name(), sampleApp.MetricsEndpoint(), sampleApp.MetricsPort()))
	//
	//	fmt.Printf("[k8sObjects], pipelineName: %v, istiofiedPipelineName: %v\n", pipelineName, istiofiedPipelineName)
	//
	//	return objs
	//}

	Context("App with istio-sidecar", Ordered, func() {
		//BeforeAll(func() {
		//	k8sObjects := makeResources()
		//
		//	//DeferCleanup(func() {
		//	//	Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		//	//})
		//	Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		//})
		//
		//It("Should have a trace backend running", func() {
		//	fmt.Printf("number 1\n")
		//	verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName, Namespace: mockNs})
		//	verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockIstiofiedBackendName, Namespace: mockIstiofiedNS})
		//})
		//
		//It("Should have sample app running with Istio sidecar", func() {
		//	fmt.Printf("number 2\n")
		//	Eventually(func(g Gomega) {
		//		listOptions := client.ListOptions{
		//			LabelSelector: labels.SelectorFromSet(map[string]string{"app": "sample-metrics"}),
		//			Namespace:     istiofiedSampleAppNs,
		//		}
		//
		//		ready, err := verifiers.IsPodReady(ctx, k8sClient, listOptions)
		//		fmt.Printf("READY: %v, err: %v\n", ready, err)
		//		g.Expect(err).NotTo(HaveOccurred())
		//		g.Expect(ready).To(BeTrue())
		//
		//		hasIstioSidecar, err := verifiers.HasContainer(ctx, k8sClient, listOptions, "istio-proxy")
		//		fmt.Printf("[ISTIO_PROXY]READY: %v, err: %v", ready, err)
		//		g.Expect(err).NotTo(HaveOccurred())
		//		g.Expect(hasIstioSidecar).To(BeTrue())
		//	}, periodic.EventuallyTimeout*2, periodic.DefaultInterval).Should(Succeed())
		//})

		//It("Should have sample app without istio sidecar", func() {
		//	fmt.Printf("number 2.1")
		//	Eventually(func(g Gomega) {
		//		listOptions := client.ListOptions{
		//			LabelSelector: labels.SelectorFromSet(map[string]string{"app": "sample-metrics"}),
		//			Namespace:     sampleAppNs,
		//		}
		//
		//		ready, err := verifiers.IsPodReady(ctx, k8sClient, listOptions)
		//		fmt.Printf("READY: %v, err: %v\n", ready, err)
		//		g.Expect(err).NotTo(HaveOccurred())
		//		g.Expect(ready).To(BeTrue())
		//	}, periodic.EventuallyTimeout*2, periodic.DefaultInterval).Should(Succeed())
		//})
		//
		//It("Should have a running trace collector deployment", func() {
		//	fmt.Printf("number 3\n")
		//	verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.TraceGatewayName)
		//})
		//
		//It("Should have the trace pipeline running", func() {
		//	fmt.Printf("number 4, pipelineName: %v, istiofiedPipelineName: %v\n", pipelineName, istiofiedPipelineName)
		//	verifiers.TracePipelineShouldBeRunning(ctx, k8sClient, pipelineName)
		//	verifiers.TracePipelineShouldBeRunning(ctx, k8sClient, istiofiedPipelineName)
		//})
		//
		//It("Trace collector with should answer requests", func() {
		//	fmt.Printf("number 5\n")
		//	By("Calling metrics service", func() {
		//		Eventually(func(g Gomega) {
		//			resp, err := proxyClient.Get(urls.Metrics())
		//			g.Expect(err).NotTo(HaveOccurred())
		//			g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		//		}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		//	})
		//})

		//It("Should have istio-proxy spans in the backend", func() {
		//	fmt.Printf("number 6\n")
		//	By("Sending http requests", func() {
		//		for _, podURLs := range urls.MultiMetricPodURL() {
		//			for i := 0; i < 100; i++ {
		//				Eventually(func(g Gomega) {
		//					resp, err := proxyClient.Get(podURLs)
		//					fmt.Printf("RESP: %v, URL: %v", resp.Status, podURLs)
		//					g.Expect(err).NotTo(HaveOccurred())
		//					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		//				}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		//			}
		//		}
		//
		//		//for i := 0; i < 100; i++ {
		//		//	Eventually(func(g Gomega) {
		//		//		resp, err := proxyClient.Get(urls.MetricPodURL())
		//		//		fmt.Printf("RESP: %v, URL: %v", resp.Status, urls.MetricPodURL())
		//		//		g.Expect(err).NotTo(HaveOccurred())
		//		//		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		//		//	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		//		//}
		//	})
		//
		//	Eventually(func(g Gomega) {
		//		resp, err := proxyClient.Get(urls.MockBackendExport(mockBackendName))
		//		g.Expect(err).NotTo(HaveOccurred())
		//		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		//		g.Expect(resp).To(HaveHTTPBody(ContainTd(
		//			// Identify istio-proxy traces by component=proxy attribute
		//			ContainSpan(WithSpanAttrs(HaveKeyWithValue("component", "proxy"))),
		//		)))
		//	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		//})

		It("Should have custom spans in the backend", func() {
			mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeTraces)
			urls.SetMockBackendExport(mockBackend.Name(), mockBackend.TelemetryExportURL(proxyClient))

			fmt.Printf("number 7\n")
			//Eventually(func(g Gomega) {
			//	fmt.Printf("url: %v\n", urls.MockBackendExport(mockBackendName))
			//	resp, err := proxyClient.Get(urls.MockBackendExport(mockBackendName))
			//	fmt.Printf("response: %v, error: %v\n", resp, err)
			//	fmt.Println("2")
			//	g.Expect(err).NotTo(HaveOccurred())
			//
			//	g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			//	fmt.Printf("response: %v\n", resp)
			//	g.Expect(resp).To(HaveHTTPBody(ContainTd(SatisfyAll(
			//		// Identify sample app by serviceName attribute
			//		ContainResourceAttrs(HaveKeyWithValue("service.name", "monitoring-custom-metrics")),
			//		ContainResourceAttrs(HaveKeyWithValue("k8s.pod.name", "istiofied-trace-emitter")),
			//		ContainResourceAttrs(HaveKeyWithValue("k8s.namespace.name", "istio-permissive-mtls")),
			//	))))
			//}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
		It("Should have custom spans in the backend from app-namespace", func() {
			mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeTraces)
			urls.SetMockBackendExport(mockBackend.Name(), mockBackend.TelemetryExportURL(proxyClient))
			verifyCustomAppSpans(urls.MockBackendExport(mockBackendName))
			fmt.Printf("number 7\n")
			//Eventually(func(g Gomega) {
			//	fmt.Printf("url: %v\n", urls.MockBackendExport(mockBackendName))
			//	resp, err := proxyClient.Get(urls.MockBackendExport(mockBackendName))
			//	fmt.Printf("response: %v, error: %v\n", resp, err)
			//	fmt.Println("2")
			//	g.Expect(err).NotTo(HaveOccurred())
			//
			//	g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			//	fmt.Printf("response: %v\n", resp)
			//	g.Expect(resp).To(HaveHTTPBody(ContainTd(SatisfyAll(
			//		// Identify sample app by serviceName attribute
			//		ContainResourceAttrs(HaveKeyWithValue("service.name", "monitoring-custom-metrics")),
			//		ContainResourceAttrs(HaveKeyWithValue("k8s.pod.name", "trace-emitter")),
			//		ContainResourceAttrs(HaveKeyWithValue("k8s.namespace.name", "app-namespace")),
			//	))))
			//}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})

func verifyCustomIstiofiedAppSpans(backendURL string) {
	Eventually(func(g Gomega) {
		fmt.Printf("url: %v\n", backendURL)
		resp, err := proxyClient.Get(backendURL)
		fmt.Printf("response: %v, error: %v\n", resp, err)
		fmt.Println("2")
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		fmt.Printf("response: %v\n", resp)
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
		fmt.Printf("url: %v\n", backendURL)
		resp, err := proxyClient.Get(backendURL)
		fmt.Printf("response: %v, error: %v\n", resp, err)
		fmt.Println("2")
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		fmt.Printf("response: %v\n", resp)
		g.Expect(resp).To(HaveHTTPBody(ContainTd(SatisfyAll(
			// Identify sample app by serviceName attribute
			ContainResourceAttrs(HaveKeyWithValue("service.name", "monitoring-custom-metrics")),
			ContainResourceAttrs(HaveKeyWithValue("k8s.pod.name", "trace-emitter")),
			ContainResourceAttrs(HaveKeyWithValue("k8s.namespace.name", "app-namespace")),
		))))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}
