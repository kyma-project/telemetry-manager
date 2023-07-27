//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"

	kitk8s "github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s"
	kitlog "github.com/kyma-project/telemetry-manager/test/e2e/testkit/kyma/telemetry/log"

	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s/verifiers"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/mocks"

	. "github.com/kyma-project/telemetry-manager/test/e2e/testkit/matchers"
)

var _ = Describe("Logging", Label("logging"), func() {

	Context("Keep labels, drop annotations", Ordered, func() {
		var (
			urls               *mocks.URLProvider
			mockNs             = "log-keep-label-mocks"
			mockDeploymentName = "log-receiver-label"
		)

		BeforeAll(func() {
			k8sObjects, logsURLProvider := makeLogsLabelTestK8sObjects(mockNs, mockDeploymentName)
			urls = logsURLProvider
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a log backend running", func() {
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: mockDeploymentName, Namespace: mockNs}
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, key)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout*2, interval).Should(Succeed())
		})

		It("Should have a log spammer running", func() {
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: mockDeploymentName + "-spammer", Namespace: mockNs}
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, key)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout*2, interval).Should(Succeed())
		})

		It("Should collect only labels and drop annotations", func() {
			Consistently(func(g Gomega) {
				time.Sleep(20 * time.Second)
				resp, err := proxyClient.Get(urls.MockBackendExport())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					HasKubernetesLabels(), Not(HasKubernetesAnnotations()))))
			}, timeout, interval).Should(Succeed())
		})

	})
})

func makeLogsLabelTestK8sObjects(namespace string, mockDeploymentName string) ([]client.Object, *mocks.URLProvider) {
	var (
		objs []client.Object
		urls = mocks.NewURLProvider()

		grpcOTLPPort = 4317
		httpOTLPPort = 4318
		httpWebPort  = 80
		httpLogPort  = 9880
	)
	mocksNamespace := kitk8s.NewNamespace(namespace)
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
	mockLogSpammer := mockHTTPBackend.LogSpammer()
	// Default namespace objects.
	logEndpointURL := mockBackendExternalService.Host()
	hostSecret := kitk8s.NewOpaqueSecret("log-rcv-hostname", defaultNamespaceName, kitk8s.WithStringData("log-host", logEndpointURL))
	logHTTPPipeline := kitlog.NewHTTPPipeline("pipeline-label-test", hostSecret.SecretKeyRef("log-host"))
	logHTTPPipeline.KeepAnnotations(false)
	logHTTPPipeline.DropLabels(false)

	objs = append(objs, []client.Object{
		mockBackendConfigMap.K8sObject(),
		mockFluentDConfigMap.K8sObjectFluentDConfig(),
		mockBackendDeployment.K8sObjectHTTP(kitk8s.WithLabel("app", mockHTTPBackend.Name())),
		mockBackendExternalService.K8sObject(kitk8s.WithLabel("app", mockHTTPBackend.Name())),
		hostSecret.K8sObject(),
		logHTTPPipeline.K8sObjectHTTP(),
		mockLogSpammer.K8sObject(kitk8s.WithLabel("app", "logging-annotation-test")),
	}...)

	urls.SetMockBackendExportAt(proxyClient.ProxyURLForService(mocksNamespace.Name(), mockHTTPBackend.Name(), telemetryDataFilename, httpWebPort), 0)

	return objs, urls
}
