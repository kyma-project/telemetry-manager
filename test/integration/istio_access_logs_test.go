//go:build e2e

package integration

import (
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/verifiers"
	kitlog "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/log"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"fmt"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks"
)

var (
	telemetryFluentbitName              = "telemetry-fluent-bit"
	telemetryWebhookEndpoint            = "telemetry-operator-webhook"
	telemetryFluentbitMetricServiceName = "telemetry-fluent-bit-metrics"
)

var _ = Describe("Istio access logs", Label("istio"), func() {
	Context("Istio", Ordered, func() {
		var (
			urls               *mocks.URLProvider
			mockNs             = "istio-access-logs-mocks"
			mockDeploymentName = "istio-access-logs-backend"
			sampleAppNs        = "sample"
		)
		BeforeAll(func() {
			k8sObjects, urlProvider := makeIstioAccessLogsK8sObjects(mockNs, mockDeploymentName, sampleAppNs)
			urls = urlProvider
			DeferCleanup(func() {
				//Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a log backend running", func() {
			fmt.Printf("aaaaaaa \n")
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: mockDeploymentName, Namespace: mockNs}
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, key)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout*2, interval).Should(Succeed())
		})

		It("Should have sample app running", func() {
			Eventually(func(g Gomega) {
				//key := types.NamespacedName{Name: "sample-metrics", Namespace: sampleAppNs}
				listOptions := client.ListOptions{
					LabelSelector: labels.SelectorFromSet(map[string]string{"app": "sample-mterics"}),
					Namespace:     sampleAppNs,
				}
				ready, err := verifiers.IsPodReady(ctx, k8sClient, listOptions)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout*2, interval).Should(Succeed())
		})

		It("Should invoke the metrics endpoint to generate access logs", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MetricPodUrl())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			}, timeout, interval).Should(Succeed())

		})

	})

})

func makeIstioAccessLogsK8sObjects(mockNs, mockDeploymentName, sampleAppNs string) ([]client.Object, *mocks.URLProvider) {
	var (
		objs []client.Object
		urls = mocks.NewURLProvider()

		grpcOTLPPort = 4317
		httpOTLPPort = 4318
		httpWebPort  = 80
		httpLogPort  = 9880
	)

	mocksNamespace := kitk8s.NewNamespace(mockNs)

	objs = append(objs, mocksNamespace.K8sObject())

	//// Mocks namespace objects.
	mockHTTPBackend := mocks.NewHTTPBackend(mockDeploymentName, mocksNamespace.Name(), "/logs/"+telemetryDataFilename)

	mockBackendConfigMap := mockHTTPBackend.HTTPBackendConfigMap("log-receiver-config")
	mockFluentDConfigMap := mockHTTPBackend.FluentDConfigMap("log-receiver-config-fluentd")
	mockBackendDeployment := mockHTTPBackend.HTTPDeployment(mockBackendConfigMap.Name(), mockFluentDConfigMap.FluentDName())
	mockBackendExternalService := mockHTTPBackend.ExternalService().
		WithPort("grpc-otlp", grpcOTLPPort).
		WithPort("http-otlp", httpOTLPPort).
		WithPort("http-web", httpWebPort).
		WithPort("http-log", httpLogPort)

	logEndpointURL := mockBackendExternalService.Host()
	hostSecret := kitk8s.NewOpaqueSecret("log-rcv-hostname", defaultNamespaceName, kitk8s.WithStringData("log-host", logEndpointURL))
	logHTTPPipeline := kitlog.NewHTTPPipeline("pipeline-istio-access-logs", hostSecret.SecretKeyRef("log-host"))
	logHTTPPipeline.WithIncludeContainer([]string{"istio-proxy"})

	// Abusing metrics provider for istio access logs
	sampleApp := mocks.NewCustomMetricProvider(sampleAppNs)

	objs = append(objs, []client.Object{
		mockBackendConfigMap.K8sObject(),
		mockFluentDConfigMap.K8sObjectFluentDConfig(),
		mockBackendDeployment.K8sObjectHTTP(kitk8s.WithLabel("app", mockHTTPBackend.Name())),
		mockBackendExternalService.K8sObject(kitk8s.WithLabel("app", mockHTTPBackend.Name())),
		sampleApp.K8sObject(),
		hostSecret.K8sObject(),
		logHTTPPipeline.K8sObjectHTTP(),
	}...)
	urls.SetMockBackendExportAt(proxyClient.ProxyURLForService(mocksNamespace.Name(), mockHTTPBackend.Name(), telemetryDataFilename, httpWebPort), 0)
	urls.SetMetricPodUrl(proxyClient.ProxyURLForPod(sampleAppNs, "sample-metrics", "/metrics/", 8080))
	return objs, urls
}
