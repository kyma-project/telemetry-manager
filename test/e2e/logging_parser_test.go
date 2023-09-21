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

const configParser = `Format regex
Regex  ^(?<user>[^ ]*) (?<pass>[^ ]*)$
Time_Key time
Time_Format %d/%b/%Y:%H:%M:%S %z
Types user:string pass:string`

var _ = Describe("Logging", Label("logging"), func() {

	Context("When a LogParser exists", Ordered, func() {
		var (
			urls               *urlprovider.URLProvider
			mockNs             = "log-parser-mocks"
			mockDeploymentName = "log-receiver-parser"
			logProducerName    = "log-producer"
		)

		BeforeAll(func() {
			k8sObjects, logsURLProvider := makeLogsRegExTestK8sObjects(mockNs, mockDeploymentName, logProducerName)
			urls = logsURLProvider
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a log backend running", func() {
			logBackendShouldBeRunning(mockDeploymentName, mockNs)
		})

		It("Should have a log producer running", func() {
			logBackendShouldBeRunning(logProducerName, mockNs)
		})

		It("Should parse the logs using regex", func() {
			Eventually(func(g Gomega) {
				time.Sleep(20 * time.Second)
				resp, err := proxyClient.Get(urls.MockBackendExport(mockDeploymentName))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainLogs(WithAttributeKeyValue("user", "foo")),
					ContainLogs(WithAttributeKeyValue("pass", "bar")),
				)))
			}, timeout, interval).Should(Succeed())
		})
	})
})

func makeLogsRegExTestK8sObjects(namespace string, mockDeploymentName, logProducerName string) ([]client.Object, *urlprovider.URLProvider) {
	var (
		objs []client.Object
		urls = urlprovider.New()
	)
	mocksNamespace := kitk8s.NewNamespace(namespace)
	objs = append(objs, mocksNamespace.K8sObject())

	// Mocks namespace objects.
	mockBackend := backend.New(mocksNamespace.Name(), mockDeploymentName, backend.SignalTypeLogs).Build()
	mockLogProducer := logproducer.New(logProducerName, mocksNamespace.Name()).WithAnnotations(map[string]string{
		"fluentbit.io/parser": "my-regex-parser",
	})
	objs = append(objs, mockBackend.K8sObjects()...)
	objs = append(objs, mockLogProducer.K8sObject(kitk8s.WithLabel("app", "regex-parser-testing-service")))
	urls.SetMockBackendExport(mockBackend.Name(), proxyClient.ProxyURLForService(
		namespace, mockBackend.Name(), backend.TelemetryDataFilename, backend.HTTPWebPort),
	)

	// Default namespace objects.
	logHTTPPipeline := kitlog.NewPipeline("pipeline-regex-parser").WithSecretKeyRef(mockBackend.GetHostSecretRefKey()).WithHTTPOutput()
	logRegExParser := kitlog.NewParser("my-regex-parser", configParser)
	objs = append(objs, logHTTPPipeline.K8sObject())
	objs = append(objs, logRegExParser.K8sObject())

	return objs, urls
}
