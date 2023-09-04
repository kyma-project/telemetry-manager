//go:build istio

package istio

import (
	"fmt"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/curljob"
	"net/http"
	"strings"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/verifiers"
	kittrace "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/trace"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/metricproducer"
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
			mockNs                 = "istio-tracing-mock"
			mockDeploymentName     = "istio-tracing-backend"
			sampleAppNs            = "istio-tracing-sample-app"
			traceCollectorBaseName = "telemetry-trace-collector"
		)
		var (
			urls               *mocks.URLProvider
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

		It("Should have a curl job completed", func() {
			Eventually(func(g Gomega) {
				listOptions := client.ListOptions{
					LabelSelector: labels.SelectorFromSet(map[string]string{"app": "sample-curl-job"}),
					Namespace:     sampleAppNs,
				}

				completed, err := verifiers.IsJobCompleted(ctx, k8sClient, listOptions)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(completed).To(BeTrue())
			}, timeout*2, interval).Should(Succeed())
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

		It("Should have istio-proxy traces in the backend", func() {

			// Identify istio-proxy traces by component=proxy attribute
			attrs := pcommon.NewMap()
			attrs.PutStr("component", "proxy")

			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainSpansWithAttributes(attrs))))
			}, timeout, telemetryDeliveryInterval).Should(Succeed())
		})
	})
})

func makeIstioTracingK8sObjects(mockNs, mockDeploymentName, sampleAppNs string) ([]client.Object, *mocks.URLProvider, string) {
	var (
		objs []client.Object
		urls = mocks.NewURLProvider()

		grpcOTLPPort    = 4317
		httpOTLPPort    = 4318
		httpWebPort     = 80
		httpMetricsPort = 8888
	)

	mocksNamespace := kitk8s.NewNamespace(mockNs)
	objs = append(objs, mocksNamespace.K8sObject())

	appNamespace := kitk8s.NewNamespace(sampleAppNs, kitk8s.WithIstioInjection())
	objs = append(objs, appNamespace.K8sObject())

	// Mocks namespace objects.
	mockBackend := mocks.NewBackend(mockDeploymentName, mocksNamespace.Name(), "/traces/"+telemetryDataFilename, mocks.SignalTypeTraces)
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
	sampleApp := metricproducer.New(sampleAppNs, "sample-producer")
	sampleCurl := curljob.New("sample-curl", sampleAppNs)

	sampleCurl.SetRepeat(100)
	sampleCurl.SetURL(fmt.Sprintf("http://%s.%s:%d/%s", sampleApp.Name(), sampleAppNs, sampleApp.MetricsPort(), strings.TrimLeft(sampleApp.MetricsEndpoint(), "/")))

	objs = append(objs, []client.Object{
		mockBackendConfigMap.K8sObject(),
		mockBackendDeployment.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
		mockBackendExternalService.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
		sampleApp.Pod().K8sObject(),
		sampleApp.Service().K8sObject(),
		hostSecret.K8sObject(),
		istioTracePipeline.K8sObject(),
		sampleCurl.K8sObject(),
		traceGatewayExternalService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", "telemetry-trace-collector")),
	}...)
	urls.SetMockBackendExportAt(proxyClient.ProxyURLForService(mocksNamespace.Name(), mockBackend.Name(), telemetryDataFilename, httpWebPort), 0)
	urls.SetMetricPodURL(proxyClient.ProxyURLForPod(sampleAppNs, sampleApp.Name(), sampleApp.MetricsEndpoint(), sampleApp.MetricsPort()))
	return objs, urls, istioTracePipeline.Name()
}
