//go:build istio

package istio

import (
	"net/http"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/verifiers"
	"github.com/kyma-project/telemetry-manager/test/testkit/kyma/istio"
	kitlog "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/metricproducer"
)

var _ = Describe("Istio access logs", Label("logging"), func() {
	Context("Istio", Ordered, func() {
		var (
			urls               *urlprovider.URLProvider
			mockNs             = "istio-access-logs-mocks"
			mockDeploymentName = "istio-access-logs-backend"
			//creating mocks in a specially prepared namespace that allows calling workloads in the mesh via API server proxy
			sampleAppNs     = "istio-permissive-mtls"
			logPipelineName string
		)
		BeforeAll(func() {
			k8sObjects, urlProvider, logPipeline := makeIstioAccessLogsK8sObjects(mockNs, mockDeploymentName, sampleAppNs)
			urls = urlProvider
			logPipelineName = logPipeline
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

		It("Should have sample app running", func() {
			Eventually(func(g Gomega) {
				listOptions := client.ListOptions{
					LabelSelector: labels.SelectorFromSet(map[string]string{"app": "sample-metrics"}),
					Namespace:     sampleAppNs,
				}
				ready, err := verifiers.IsPodReady(ctx, k8sClient, listOptions)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout*2, interval).Should(Succeed())
		})

		It("Should have the log pipeline running", func() {
			Eventually(func(g Gomega) bool {
				var pipeline telemetryv1alpha1.LogPipeline
				key := types.NamespacedName{Name: logPipelineName}
				g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
				return pipeline.Status.HasCondition(telemetryv1alpha1.LogPipelineRunning)
			}, timeout, interval).Should(BeTrue())
		})

		It("Should invoke the metrics endpoint to generate access logs", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MetricPodURL())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			}, timeout, interval).Should(Succeed())
		})

		It("Should verify istio logs are present", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainLogs(WithAttributeKeys(istio.AccessLogAttributeKeys...)))))
			}, timeout, interval).Should(Succeed())
		})
	})
})

func makeIstioAccessLogsK8sObjects(mockNs, mockDeploymentName, sampleAppNs string) ([]client.Object, *urlprovider.URLProvider, string) {
	var (
		objs []client.Object
		urls = urlprovider.New()

		grpcOTLPPort = 4317
		httpOTLPPort = 4318
		httpWebPort  = 80
		httpLogPort  = 9880
	)

	mocksNamespace := kitk8s.NewNamespace(mockNs)
	objs = append(objs, mocksNamespace.K8sObject())

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

	logEndpointURL := mockBackendExternalService.Host()
	hostSecret := kitk8s.NewOpaqueSecret("log-rcv-hostname", defaultNamespaceName, kitk8s.WithStringData("log-host", logEndpointURL))
	istioAccessLogsPipeline := kitlog.NewPipeline("pipeline-istio-access-logs").WithSecretKeyRef(hostSecret.SecretKeyRef("log-host")).WithIncludeContainer([]string{"istio-proxy"}).WithHTTPOutput()

	// Abusing metrics provider for istio access logs
	sampleApp := metricproducer.New(sampleAppNs, metricproducer.WithName("access-log-emitter"))

	objs = append(objs, []client.Object{
		mockBackendConfigMap.K8sObject(),
		mockFluentdConfigMap.K8sObject(),
		mockBackendDeployment.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
		mockBackendExternalService.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
		sampleApp.Pod().K8sObject(),
		hostSecret.K8sObject(),
		istioAccessLogsPipeline.K8sObject(),
	}...)
	urls.SetMockBackendExport(proxyClient.ProxyURLForService(mocksNamespace.Name(), mockBackend.Name(), telemetryDataFilename, httpWebPort), 0)
	urls.SetMetricPodURL(proxyClient.ProxyURLForPod(sampleAppNs, sampleApp.Name(), sampleApp.MetricsEndpoint(), sampleApp.MetricsPort()))
	return objs, urls, istioAccessLogsPipeline.Name()
}
