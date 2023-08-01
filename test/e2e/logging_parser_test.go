//go:build e2e

package e2e

import (
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitlog "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/log"

	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/verifiers"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks"

	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers"
)

var (
	telemetryFluentbitName              = "telemetry-fluent-bit"
	telemetryWebhookEndpoint            = "telemetry-operator-webhook"
	telemetryFluentbitMetricServiceName = "telemetry-fluent-bit-metrics"
)

const configParser = `Format regex
Regex  ^(?<user>[^ ]*) (?<pass>[^ ]*)$
Time_Key time
Time_Format %d/%b/%Y:%H:%M:%S %z
Types user:string pass:string`

var _ = Describe("Logging", Label("logging"), func() {

	Context("LogParser", Ordered, func() {
		var (
			urls               *mocks.URLProvider
			mockNs             = "log-parser-mocks"
			mockDeploymentName = "log-receiver-parser"
		)

		BeforeAll(func() {
			k8sObjects, logsURLProvider := makeLogsRegExTestK8sObjects(mockNs, mockDeploymentName)
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

		It("Should parse the logs using regex", func() {
			Eventually(func(g Gomega) {
				time.Sleep(20 * time.Second)
				resp, err := proxyClient.Get(urls.MockBackendExport())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainsLogsKeyValue("user", "foo"), ContainsLogsKeyValue("pass", "bar"))))
			}, timeout, interval).Should(Succeed())
		})
	})
})

func makeLogsRegExTestK8sObjects(namespace string, mockDeploymentName string) ([]client.Object, *mocks.URLProvider) {
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
	logHTTPPipeline := kitlog.NewHTTPPipeline("pipeline-regex-parser", hostSecret.SecretKeyRef("log-host"))
	logRegExParser := kitlog.NewParser("my-regex-parser", configParser)

	mockLogSpammer.WithParser("my-regex-parser")
	objs = append(objs, []client.Object{
		mockBackendConfigMap.K8sObject(),
		mockFluentDConfigMap.K8sObjectFluentDConfig(),
		mockBackendDeployment.K8sObjectHTTP(kitk8s.WithLabel("app", mockHTTPBackend.Name())),
		mockBackendExternalService.K8sObject(kitk8s.WithLabel("app", mockHTTPBackend.Name())),
		hostSecret.K8sObject(),
		logHTTPPipeline.K8sObjectHTTP(),
		mockLogSpammer.K8sObject(kitk8s.WithLabel("app", "regex-parser-testing-service")),
		logRegExParser.K8sObject(),
	}...)

	urls.SetMockBackendExportAt(proxyClient.ProxyURLForService(mocksNamespace.Name(), mockHTTPBackend.Name(), telemetryDataFilename, httpWebPort), 0)

	return objs, urls
}
