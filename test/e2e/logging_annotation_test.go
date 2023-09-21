//go:build e2e

package e2e

import (
	"net/http"
	"time"

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

	Context("Keep annotations, drop labels", Ordered, func() {
		var (
			urls               *urlprovider.URLProvider
			mockNs             = "log-keep-anno-mocks"
			mockDeploymentName = "log-receiver-annotation"
			logProducerName    = "log-producer"
		)

		BeforeAll(func() {
			k8sObjects, logsURLProvider := makeLogsAnnotationTestK8sObjects(mockNs, mockDeploymentName, logProducerName)
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

		It("Should collect only annotations and drop label", func() {
			Consistently(func(g Gomega) {
				time.Sleep(20 * time.Second)
				resp, err := proxyClient.Get(urls.MockBackendExport(mockDeploymentName))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainLogs(WithKubernetesAnnotations("release", "v1.0.0")),
					Not(ContainLogs(WithKubernetesLabels())),
				)))
			}, telemetryDeliveryTimeout, interval).Should(Succeed())
		})
	})
})

func makeLogsAnnotationTestK8sObjects(mockNs, mockDeploymentName, logProducerName string) ([]client.Object, *urlprovider.URLProvider) {
	var (
		objs []client.Object
		urls = urlprovider.New()
	)
	objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

	// Mock namespace objects.
	mockBackend, err := backend.New(mockDeploymentName, mockNs, backend.SignalTypeLogs)
	Expect(err).NotTo(HaveOccurred())
	mockLogProducer := logproducer.New(logProducerName, mockNs).
		WithAnnotations(map[string]string{"release": "v1.0.0"})
	objs = append(objs, mockLogProducer.K8sObject(kitk8s.WithLabel("app", "logging-annotation-test")))
	objs = append(objs, mockBackend.K8sObjects()...)
	urls.SetMockBackendExport(mockBackend.Name(), proxyClient.ProxyURLForService(
		mockNs, mockBackend.Name(), backend.TelemetryDataFilename, backend.HTTPWebPort),
	)

	// Default namespace objects.
	logPipeline := kitlog.NewPipeline("pipeline-annotation-test").
		WithSecretKeyRef(mockBackend.HostSecretRefKey()).
		WithHTTPOutput()
	logPipeline.KeepAnnotations(true)
	logPipeline.DropLabels(true)
	objs = append(objs, logPipeline.K8sObject())

	return objs, urls
}
