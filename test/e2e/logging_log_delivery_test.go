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

type OutputType string

const (
	OutputTypeHTTP   = "http"
	OutputTypeCustom = "custom"
)

var _ = Describe("Logging", Label("logging"), func() {

	Context("When a logpipeline with HTTP output exists", Ordered, func() {
		var (
			urls               *urlprovider.URLProvider
			mockDeploymentName = "log-receiver"
			mockNs             = "log-http-output"
			logProducerName    = "log-producer-http-output" //#nosec G101 -- This is a false positive
		)

		BeforeAll(func() {
			k8sObjects, logsURLProvider := makeLogDeliveryTestK8sObjects(mockNs, mockDeploymentName, logProducerName, OutputTypeHTTP)
			urls = logsURLProvider
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a log backend running", Label("operational"), func() {
			deploymentShouldBeReady(mockDeploymentName, mockNs)
		})

		It("Should have a log producer running", func() {
			deploymentShouldBeReady(logProducerName, mockNs)
		})

		It("Should verify end-to-end log delivery with http ", Label("operational"), func() {
			logsShouldBeDelivered(logProducerName, urls.MockBackendExport(mockDeploymentName))
		})
	})

	Context("When a logpipeline with custom output exists", Ordered, func() {
		var (
			urls               *urlprovider.URLProvider
			mockDeploymentName = "log-receiver"
			mockNs             = "log-custom-output"
			logProducerName    = "log-producer-custom-output" //#nosec G101 -- This is a false positive
		)

		BeforeAll(func() {
			k8sObjects, logsURLProvider := makeLogDeliveryTestK8sObjects(mockNs, mockDeploymentName, logProducerName, OutputTypeCustom)
			urls = logsURLProvider
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a log backend running", func() {
			deploymentShouldBeReady(mockDeploymentName, mockNs)
		})

		It("Should verify end-to-end log delivery with custom output", func() {
			logsShouldBeDelivered(logProducerName, urls.MockBackendExport(mockDeploymentName))
		})
	})
})

func logsShouldBeDelivered(logProducerName string, proxyURL string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(proxyURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
			ContainLogs(WithPod(logProducerName)))))
	}, timeout, interval).Should(Succeed())
}

func makeLogDeliveryTestK8sObjects(mockNs string, mockDeploymentName string, logProducerName string, outputType OutputType) ([]client.Object, *urlprovider.URLProvider) {
	var (
		objs []client.Object
		urls = urlprovider.New()
	)

	objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

	//// Mocks namespace objects.
	mockBackend := backend.New(mockDeploymentName, mockNs, backend.SignalTypeLogs)
	mockLogProducer := logproducer.New(logProducerName, mockNs)
	objs = append(objs, mockBackend.K8sObjects()...)
	objs = append(objs, mockLogProducer.K8sObject(kitk8s.WithLabel("app", "logging-test")))
	urls.SetMockBackendExport(mockBackend.Name(), proxyClient.ProxyURLForService(
		mockNs, mockBackend.Name(), backend.TelemetryDataFilename, backend.HTTPWebPort),
	)

	// Default namespace objects.
	var logPipeline *kitlog.Pipeline
	if outputType == OutputTypeHTTP {
		logPipeline = kitlog.NewPipeline("http-output-pipeline").WithSecretKeyRef(mockBackend.HostSecretRefKey()).WithHTTPOutput()
	} else {
		logPipeline = kitlog.NewPipeline("custom-output-pipeline").WithCustomOutput(mockBackend.ExternalService.Host()) // TODO check if it makes sense to extract the host into a Backend function
	}
	objs = append(objs, logPipeline.K8sObject())

	return objs, urls
}
