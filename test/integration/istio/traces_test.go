//go:build istio

package istio

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/trace"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelIntegration), Ordered, func() {
	const (
		appName          = "app-1"
		istiofiedAppName = "app-2"
		istiofiedAppNs   = "istio-permissive-mtls" // creating mocks in a specially prepared namespace that allows calling workloads in the mesh via API server proxy
	)

	var (
		backendNs                 = suite.ID()
		istiofiedBackendNs        = suite.IDWithSuffix("istiofied")
		appNs                     = suite.IDWithSuffix("app")
		pipeline1Name             = suite.IDWithSuffix("1")
		pipeline2Name             = suite.IDWithSuffix("2")
		backendExportURL          string
		istiofiedBackendExportURL string
		appURL                    string
		istiofiedAppURL           string
		metricServiceURL          string
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(backendNs).K8sObject())
		objs = append(objs, kitk8s.NewNamespace(istiofiedBackendNs, kitk8s.WithIstioInjection()).K8sObject())
		objs = append(objs, kitk8s.NewNamespace(appNs).K8sObject())

		backend1 := backend.New(backendNs, backend.SignalTypeTraces)
		objs = append(objs, backend1.K8sObjects()...)
		backendExportURL = backend1.ExportURL(proxyClient)

		backend2 := backend.New(istiofiedBackendNs, backend.SignalTypeTraces)
		objs = append(objs, backend2.K8sObjects()...)
		istiofiedBackendExportURL = backend2.ExportURL(proxyClient)

		istioTracePipeline := testutils.NewTracePipelineBuilder().
			WithName(pipeline2Name).
			WithOTLPOutput(testutils.OTLPEndpoint(backend2.Endpoint())).
			Build()
		objs = append(objs, &istioTracePipeline)

		tracePipeline := testutils.NewTracePipelineBuilder().
			WithName(pipeline1Name).
			WithOTLPOutput(testutils.OTLPEndpoint(backend1.Endpoint())).
			Build()
		objs = append(objs, &tracePipeline)

		traceGatewayExternalService := kitk8s.NewService("telemetry-otlp-traces-external", kitkyma.SystemNamespaceName).
			WithPort("grpc-otlp", ports.OTLPGRPC).
			WithPort("http-metrics", ports.Metrics)
		metricServiceURL = proxyClient.ProxyURLForService(kitkyma.SystemNamespaceName, "telemetry-otlp-traces-external", "metrics", ports.Metrics)
		objs = append(objs, traceGatewayExternalService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", "telemetry-trace-gateway")))

		// Abusing metrics provider for istio traces
		istiofiedApp := prommetricgen.New(istiofiedAppNs, prommetricgen.WithName(istiofiedAppName))
		objs = append(objs, istiofiedApp.Pod().K8sObject())
		istiofiedAppURL = istiofiedApp.PodURL(proxyClient)

		app := prommetricgen.New(appNs, prommetricgen.WithName(appName))
		objs = append(objs, app.Pod().K8sObject())
		appURL = app.PodURL(proxyClient)

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
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: backendNs})
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: istiofiedBackendNs})
		})

		It("Should have sample app running with Istio sidecar", func() {
			verifyAppIsRunning(istiofiedAppNs, map[string]string{"app": "sample-metrics"})
			verifySidecarPresent(istiofiedAppNs, map[string]string{"app": "sample-metrics"})
		})

		It("Should have sample app without istio sidecar", func() {
			verifyAppIsRunning(appNs, map[string]string{"app": "sample-metrics"})
		})

		It("Should have a running trace gateway deployment", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.TraceGatewayName)
		})

		It("Should have the trace pipelines running", func() {
			assert.TracePipelineHealthy(ctx, k8sClient, pipeline1Name)
			assert.TracePipelineHealthy(ctx, k8sClient, pipeline2Name)
		})

		It("Trace gateway with should answer requests", func() {
			By("Calling metrics service", func() {
				Eventually(func(g Gomega) {
					resp, err := proxyClient.Get(metricServiceURL)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
			})
		})

		It("Should invoke istiofied and non-istiofied apps", func() {
			By("Sending http requests", func() {
				for _, podURLs := range []string{appURL, istiofiedAppURL} {
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
			verifyIstioSpans(backendExportURL, istiofiedAppNs)
			verifyIstioSpans(istiofiedBackendExportURL, istiofiedAppNs)
		})
		It("Should have custom spans in the backend from istiofied workload", func() {
			verifyCustomIstiofiedAppSpans(backendExportURL, istiofiedAppName, istiofiedAppNs)
			verifyCustomIstiofiedAppSpans(istiofiedBackendExportURL, istiofiedAppName, istiofiedAppNs)
		})
		It("Should have custom spans in the backend from app-namespace", func() {
			verifyCustomAppSpans(backendExportURL, appName, appNs)
			verifyCustomAppSpans(istiofiedBackendExportURL, appName, appNs)
		})
		It("Should have no noisy spans of communication to telemetry-otlp-traces endpoint", func() {
			verifyNoIstioNoiseSpans(backendExportURL, istiofiedAppName, istiofiedAppNs)
			verifyNoIstioNoiseSpans(istiofiedBackendExportURL, istiofiedAppName, istiofiedAppNs)
		})
	})
})

func verifySidecarPresent(namespace string, labelSelector map[string]string) {
	Eventually(func(g Gomega) {
		listOptions := client.ListOptions{
			LabelSelector: labels.SelectorFromSet(labelSelector),
			Namespace:     namespace,
		}

		hasIstioSidecar, err := assert.HasContainer(ctx, k8sClient, listOptions, "istio-proxy")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(hasIstioSidecar).To(BeTrueBecause("Istio sidecar not present"))
	}, periodic.EventuallyTimeout*2, periodic.DefaultInterval).Should(Succeed())
}

func verifyAppIsRunning(namespace string, labelSelector map[string]string) {
	listOptions := client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labelSelector),
		Namespace:     namespace,
	}

	assert.PodsReady(ctx, k8sClient, listOptions)
}

func verifyIstioSpans(backendURL, namespace string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(backendURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		g.Expect(resp).To(HaveHTTPBody(HaveFlatTraces(ContainElement(SatisfyAll(
			// Identify istio-proxy traces by component=proxy attribute
			HaveSpanAttributes(HaveKeyWithValue("component", "proxy")),
			HaveSpanAttributes(HaveKeyWithValue("istio.namespace", namespace)),
		)))))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func verifyNoIstioNoiseSpans(backendURL, namespace string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(backendURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		g.Expect(resp).NotTo(HaveHTTPBody(HaveFlatTraces(ContainElement(SatisfyAll(
			// Identify istio-proxy traces by component=proxy attribute
			HaveSpanAttributes(HaveKeyWithValue("component", "proxy")),
			// All calls to telemetry-otlp-traces should be dropped
			HaveSpanAttributes(HaveKeyWithValue("http.url", "http://telemetry-otlp-traces.kyma-system.svc.cluster.local:4318/v1/traces")),
		)))))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func verifyCustomIstiofiedAppSpans(backendURL, name, namespace string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(backendURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(HaveFlatTraces(ContainElement(SatisfyAll(
			// Identify sample app by serviceName attribute
			HaveResourceAttributes(HaveKeyWithValue("service.name", "monitoring-custom-metrics")),
			HaveResourceAttributes(HaveKeyWithValue("k8s.pod.name", name)),
			HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", namespace)),
		)))))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func verifyCustomAppSpans(backendURL, name, namespace string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(backendURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(HaveFlatTraces(ContainElement(SatisfyAll(
			// Identify sample app by serviceName attribute
			HaveResourceAttributes(HaveKeyWithValue("service.name", "monitoring-custom-metrics")),
			HaveResourceAttributes(HaveKeyWithValue("k8s.pod.name", name)),
			HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", namespace)),
		)))))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}
