//go:build e2e

package e2e

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitlog "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/log"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/logproducer"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
)

var _ = Describe("Logging", Label("logging"), func() {
	Context("dedot labels", Ordered, func() {
		var (
			urls               *urlprovider.URLProvider
			mockNs             = "log-dedot-labels-mocks"
			mockDeploymentName = "log-receiver-dedot-labels"
			logProducerName    = "log-producer"
		)

		BeforeAll(func() {
			k8sObjects, logsURLProvider := makeLogsDeDotTestK8sObjects(mockNs, mockDeploymentName, logProducerName)
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

		// label foo.bar: value should be represented as foo_bar:value
		It("Should dedot the labels", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport(mockDeploymentName))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainLogs(WithKubernetesLabels("dedot_label", "logging-dedot-value"))),
				))
			}, timeout, interval).Should(Succeed())
		})
	})
})

func makeLogsDeDotTestK8sObjects(mockNs string, mockDeploymentName, logProducerName string) ([]client.Object, *urlprovider.URLProvider) {
	var (
		objs []client.Object
		urls = urlprovider.New()
	)
	objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

	//// Mocks namespace objects.
	mockBackend, err := backend.New(mockDeploymentName, mockNs, backend.SignalTypeLogs)
	Expect(err).NotTo(HaveOccurred())
	mockLogProducer := logproducer.New(logProducerName, mockNs)
	objs = append(objs, mockBackend.K8sObjects()...)
	objs = append(objs, mockLogProducer.K8sObject(kitk8s.WithLabel("dedot.label", "logging-dedot-value")))
	urls.SetMockBackendExport(mockBackend.Name(), proxyClient.ProxyURLForService(
		mockNs, mockBackend.Name(), backend.TelemetryDataFilename, backend.HTTPWebPort),
	)

	// Default namespace objects.
	logPipeline := kitlog.NewPipeline("pipeline-dedot-test").
		WithSecretKeyRef(mockBackend.HostSecretRefKey()).
		WithHTTPOutput().
		WithIncludeContainer([]string{logProducerName})
	objs = append(objs, logPipeline.K8sObject())

	return objs, urls
}
