//go:build e2e

package e2e

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/verifiers"
	kitlog "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/log"

	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/logproducer"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"

	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers"
)

var _ = Describe("Logging", Label("logging"), func() {

	Context("Log delivery to HTTP output", Ordered, func() {
		var (
			urls               *urlprovider.URLProvider
			mockDeploymentName = "log-receiver"
			namespace          = "HTTP-output-log-delivery"
		)

		BeforeAll(func() {
			k8sObjects, logsURLProvider := makeHTTPOutputLogTestK8sObjects(namespace, mockDeploymentName)
			urls = logsURLProvider
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a log backend running", Label("operational"), func() {
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: mockDeploymentName, Namespace: namespace}
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, key)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout*2, interval).Should(Succeed())
		})

		It("Should verify end-to-end log delivery", Label("operational"), func() {

			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainLogs(Any()))))
			}, timeout, interval).Should(Succeed())
		})
	})
})

func makeHTTPOutputLogTestK8sObjects(namespace string, mockDeploymentName string) ([]client.Object, *urlprovider.URLProvider) {
	var (
		objs []client.Object
		urls = urlprovider.New()

		grpcOTLPPort = 4317
		httpOTLPPort = 4318
		httpWebPort  = 80
		httpLogPort  = 9880
	)
	mocksNamespace := kitk8s.NewNamespace(namespace)

	//// Mocks namespace objects.
	mockBackend := backend.New(mockDeploymentName, mocksNamespace.Name(), "/logs/"+telemetryDataFilename, backend.SignalTypeLogs)

	mockBackendConfigMap := mockBackend.ConfigMap("log-receiver-config")
	mockFluentdConfigMap := mockBackend.FluentdConfigMap("log-receiver-config-fluentd")
	mockBackendDeployment := mockBackend.Deployment(mockBackendConfigMap.Name()).WithFluentdConfigName(mockFluentdConfigMap.Name())
	mockBackendExternalService := mockBackend.ExternalService().
		WithPort("grpc-otlp", grpcOTLPPort).
		WithPort("http-otlp", httpOTLPPort).
		WithPort("http-web", httpWebPort).
		WithPort("http-log", httpLogPort)
	mockLogProducer := logproducer.New(mocksNamespace.Name())

	// Default namespace objects.
	logEndpointURL := mockBackendExternalService.Host()
	hostSecret := kitk8s.NewOpaqueSecret("log-rcv-hostname", defaultNamespaceName, kitk8s.WithStringData("log-host", logEndpointURL))
	logPipeline := kitlog.NewPipeline("http-output-pipeline").WithSecretKeyRef(hostSecret.SecretKeyRef("log-host")).WithHTTPOutput()

	objs = append(objs, []client.Object{
		mocksNamespace.K8sObject(),
		mockBackendConfigMap.K8sObject(),
		mockFluentdConfigMap.K8sObject(),
		mockBackendDeployment.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
		mockBackendExternalService.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
		hostSecret.K8sObject(),
		logPipeline.K8sObject(),
		mockLogProducer.K8sObject(kitk8s.WithLabel("app", "logging-test")),
	}...)

	urls.SetMockBackendExportAt(proxyClient.ProxyURLForService(mocksNamespace.Name(), mockBackend.Name(), telemetryDataFilename, httpWebPort), 0)

	return objs, urls
}
