//go:build istio

package istio

import (
	"net/http"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/verifiers"
	kittrace "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/trace"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/metricproducer"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Istio tracing", Label("tracing"), func() {
	Context("App with istio-sidecar", Ordered, func() {
		const (
			mockNs             = "istio-tracing-mock"
			mockDeploymentName = "istio-tracing-backend"
			//creating mocks in a specially prepared namespace that allows calling workloads in the mesh via API server proxy
			sampleAppNs            = "istio-permissive-mtls"
			traceCollectorBaseName = "telemetry-trace-collector"
		)
		var (
			urls               *urlprovider.URLProvider
			tracePipelineName  string
			traceCollectorname = types.NamespacedName{Name: traceCollectorBaseName, Namespace: kymaSystemNamespaceName}
		)

		BeforeAll(func() {
			k8sObjects, urlProvider, tracePipeline := makeIstioTracingK8sObjects(mockNs, mockDeploymentName, sampleAppNs)
			urls = urlProvider
			tracePipelineName = tracePipeline

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a trace backend running", func() {
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: mockDeploymentName, Namespace: mockNs}
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, key)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout*2, interval).Should(Succeed())
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
			}, timeout*2, interval).Should(Succeed())
		})

		It("Should have a running trace collector deployment", func() {
			Eventually(func(g Gomega) {
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, traceCollectorname)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		It("Should have the trace pipeline running", func() {
			Eventually(func(g Gomega) bool {
				var pipeline telemetryv1alpha1.TracePipeline
				key := types.NamespacedName{Name: tracePipelineName}
				g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
				return pipeline.Status.HasCondition(telemetryv1alpha1.TracePipelineRunning)
			}, timeout, interval).Should(BeTrue())
		})

		It("Trace collector should answer requests", func() {
			By("Calling metrics service", func() {
				Eventually(func(g Gomega) {
					resp, err := proxyClient.Get(urls.Metrics())
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				}, timeout, interval).Should(Succeed())
			})
		})

		It("Should have istio-proxy spans in the backend", func() {
			By("Sending http requests", func() {
				for i := 0; i < 100; i++ {
					Eventually(func(g Gomega) {
						resp, err := proxyClient.Get(urls.MetricPodURL())
						g.Expect(err).NotTo(HaveOccurred())
						g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
					}, timeout, interval).Should(Succeed())
				}
			})

			// Identify istio-proxy traces by component=proxy attribute
			proxyAttrs := pcommon.NewMap()
			proxyAttrs.PutStr("component", "proxy")

			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainSpansWithAttributes(proxyAttrs))))
			}, timeout, interval).Should(Succeed())
		})

		It("Should have custom spans in the backend", func() {
			// Identify sample app by serviceName attribute
			customResourceAttr := pcommon.NewMap()
			customResourceAttr.PutStr("service.name", "monitoring-custom-metrics")

			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainSpansWithResourceAttributes(customResourceAttr))))
			}, timeout, interval).Should(Succeed())
		})
	})
})

func makeIstioTracingK8sObjects(mockNs, mockDeploymentName, sampleAppNs string) ([]client.Object, *urlprovider.URLProvider, string) {
	var (
		objs []client.Object
		urls = urlprovider.New()

		grpcOTLPPort    = 4317
		httpOTLPPort    = 4318
		httpWebPort     = 80
		httpMetricsPort = 8888
	)

	mocksNamespace := kitk8s.NewNamespace(mockNs)
	objs = append(objs, mocksNamespace.K8sObject())

	// Mocks namespace objects.
	mockBackend := backend.New(mockDeploymentName, mocksNamespace.Name(), "/traces/"+telemetryDataFilename, backend.SignalTypeTraces)
	mockBackendConfigMap := mockBackend.ConfigMap("trace-receiver-config")
	mockBackendDeployment := mockBackend.Deployment(mockBackendConfigMap.Name())
	mockBackendExternalService := mockBackend.ExternalService().
		WithPort("grpc-otlp", grpcOTLPPort).
		WithPort("http-otlp", httpOTLPPort).
		WithPort("http-web", httpWebPort)

	traceEndpointURL := mockBackendExternalService.OTLPEndpointURL(grpcOTLPPort)
	hostSecret := kitk8s.NewOpaqueSecret("trace-rcv-hostname", defaultNamespaceName, kitk8s.WithStringData("trace-host", traceEndpointURL))
	istioTracePipeline := kittrace.NewPipeline("pipeline-istio-traces", hostSecret.SecretKeyRef("trace-host"))

	// Kyma-system namespace objects.
	traceGatewayExternalService := kitk8s.NewService("telemetry-otlp-traces-external", kymaSystemNamespaceName).
		WithPort("grpc-otlp", grpcOTLPPort).
		WithPort("http-metrics", httpMetricsPort)
	urls.SetMetrics(proxyClient.ProxyURLForService(kymaSystemNamespaceName, "telemetry-otlp-traces-external", "metrics", httpMetricsPort))

	// Abusing metrics provider for istio traces
	sampleApp := metricproducer.New(sampleAppNs, metricproducer.WithName("trace-emitter"))

	objs = append(objs, []client.Object{
		mockBackendConfigMap.K8sObject(),
		mockBackendDeployment.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
		mockBackendExternalService.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
		sampleApp.Pod().K8sObject(),
		hostSecret.K8sObject(),
		istioTracePipeline.K8sObject(),
		traceGatewayExternalService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", "telemetry-trace-collector")),
	}...)
	urls.SetMockBackendExportAt(proxyClient.ProxyURLForService(mocksNamespace.Name(), mockBackend.Name(), telemetryDataFilename, httpWebPort), 0)
	urls.SetMetricPodURL(proxyClient.ProxyURLForPod(sampleAppNs, sampleApp.Name(), sampleApp.MetricsEndpoint(), sampleApp.MetricsPort()))
	return objs, urls, istioTracePipeline.Name()
}
