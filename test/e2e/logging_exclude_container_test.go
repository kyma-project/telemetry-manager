//go:build e2e

package e2e

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitlog "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/logproducer"

	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"

	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers"
)

var _ = Describe("Logging", Label("logging"), func() {

	Context("Container Excludes", Ordered, func() {
		var (
			urls               *urlprovider.URLProvider
			mockNs             = "log-exclude-container-mocks"
			mockDeploymentName = "log-receiver-exclude-container"
			logProducerName    = "log-producer-exclude-container"
		)

		BeforeAll(func() {
			k8sObjects, logsURLProvider := makeLogsTestExcludeContainerK8sObjects(mockNs, mockDeploymentName, logProducerName)
			urls = logsURLProvider
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a log backend running", func() {
			deploymentShouldBeReady(mockDeploymentName, mockNs)
		})

		It("Should have a log producer running", func() {
			deploymentShouldBeReady(logProducerName, mockNs)
		})

		It("Should collect logs", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport(mockDeploymentName))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainLogs(Any()))))
			}, timeout, interval).Should(Succeed())
		})

		It("Should not collect any log-producer logs", func() {
			Consistently(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport(mockDeploymentName))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					Not(ContainLogs(WithContainer(logProducerName))))))
			}, telemetryDeliveryTimeout, interval).Should(Succeed())
		})

	})
})

func makeLogsTestExcludeContainerK8sObjects(namespace string, mockDeploymentName, logProducerName string) ([]client.Object, *urlprovider.URLProvider) {
	var (
		objs []client.Object
		urls = urlprovider.New()
	)
	mocksNamespace := kitk8s.NewNamespace(namespace)
	objs = append(objs, mocksNamespace.K8sObject())

	//// Mocks namespace objects.
	mockBackend := backend.New(mocksNamespace.Name(), mockDeploymentName, backend.SignalTypeLogs).Build()
	mockLogProducer := logproducer.New(logProducerName, mocksNamespace.Name())
	objs = append(objs, mockBackend.K8sObjects()...)
	objs = append(objs, mockLogProducer.K8sObject(kitk8s.WithLabel("app", "logging-exclude-container")))
	urls.SetMockBackendExport(mockBackend.Name(), proxyClient.ProxyURLForService(
		namespace, mockBackend.Name(), backend.TelemetryDataFilename, backend.HTTPWebPort),
	)

	// Default namespace objects.
	logPipeline := kitlog.NewPipeline("pipeline-exclude-container").
		WithSecretKeyRef(mockBackend.GetHostSecretRefKey()).
		WithHTTPOutput().
		WithExcludeContainer([]string{logProducerName})
	objs = append(objs, logPipeline.K8sObject())

	return objs, urls
}
