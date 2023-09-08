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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/logproducer"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"

	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/verifiers"

	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers"
)

var _ = Describe("Logging", Label("logging"), func() {

	Context("Keep annotations, drop labels", Ordered, func() {
		var (
			urls               *urlprovider.URLProvider
			mockNs             = "log-keep-anno-mocks"
			mockDeploymentName = "log-receiver-annotation"
		)

		BeforeAll(func() {
			k8sObjects, logsURLProvider := makeLogsAnnotationTestK8sObjects(mockNs,
				backend.NewOptions(mockDeploymentName),
			)
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

		It("Should have a log producer running", func() {
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: "log-producer", Namespace: mockNs}
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, key)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout*2, interval).Should(Succeed())
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

func makeLogsAnnotationTestK8sObjects(namespace string, options *backend.Options) ([]client.Object, *urlprovider.URLProvider) {
	var (
		objs []client.Object
		urls = urlprovider.New()
	)
	mocksNamespace := kitk8s.NewNamespace(namespace)
	objs = append(objs, mocksNamespace.K8sObject())

	// Mock namespace objects.
	mockBackend := backend.New(mocksNamespace.Name(), options)
	mockLogProducer := logproducer.New("log-producer", mocksNamespace.Name()).
		WithAnnotations(map[string]string{
			"release": "v1.0.0",
		})
	// Default namespace objects.
	logPipeline := kitlog.NewPipeline("pipeline-annotation-test").WithSecretKeyRef(mockBackend.GetHostSecretRefKey()).WithHTTPOutput()
	logPipeline.KeepAnnotations(true)
	logPipeline.DropLabels(true)

	objs = append(objs, mockBackend.K8sObjects()...)
	objs = append(objs, logPipeline.K8sObject())
	objs = append(objs, mockLogProducer.K8sObject(kitk8s.WithLabel("app", "logging-annotation-test")))

	urls.SetMockBackendExport(options.Name, proxyClient.ProxyURLForService(
		namespace, options.Name, backend.TelemetryDataFilename, backend.HTTPWebPort),
	)

	return objs, urls
}
