//go:build istio

package istio

import (
	"net/http"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/kyma/istio"
	kitlog "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/metricproducer"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

var _ = Describe("Istio Access Logs", Label("logging"), func() {
	const (
		mockNs          = "istio-access-logs-mocks"
		mockBackendName = "istio-access-logs-backend"
		//creating mocks in a specially prepared namespace that allows calling workloads in the mesh via API server proxy
		sampleAppNs = "istio-permissive-mtls"
	)

	var (
		urls         = urlprovider.New()
		pipelineName string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeLogs)
		objs = append(objs, mockBackend.K8sObjects()...)
		urls.SetMockBackendExport(mockBackend.Name(), mockBackend.TelemetryExportURL(proxyClient))

		istioAccessLogsPipeline := kitlog.NewPipeline("pipeline-istio-access-logs").
			WithSecretKeyRef(mockBackend.HostSecretRef()).
			WithIncludeContainer([]string{"istio-proxy"}).
			WithHTTPOutput()
		pipelineName = istioAccessLogsPipeline.Name()
		objs = append(objs, istioAccessLogsPipeline.K8sObject())

		// Abusing metrics provider for istio access logs
		sampleApp := metricproducer.New(sampleAppNs, metricproducer.WithName("access-log-emitter"))
		objs = append(objs, sampleApp.Pod().K8sObject())
		urls.SetMetricPodURL(proxyClient.ProxyURLForPod(sampleAppNs, sampleApp.Name(), sampleApp.MetricsEndpoint(), sampleApp.MetricsPort()))

		return objs
	}

	Context("Istio", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a log backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName, Namespace: mockNs})
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
			}, periodic.Timeout*2, periodic.Interval).Should(Succeed())
		})

		It("Should have the log pipeline running", func() {
			Eventually(func(g Gomega) bool {
				var pipeline telemetryv1alpha1.LogPipeline
				key := types.NamespacedName{Name: pipelineName}
				g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
				return pipeline.Status.HasCondition(telemetryv1alpha1.LogPipelineRunning)
			}, periodic.Timeout, periodic.Interval).Should(BeTrue())
		})

		It("Should invoke the metrics endpoint to generate access logs", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MetricPodURL())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			}, periodic.Timeout, periodic.Interval).Should(Succeed())
		})

		It("Should verify istio logs are present", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport(mockBackendName))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainLogs(WithAttributeKeys(istio.AccessLogAttributeKeys...)))))
			}, periodic.TelemetryPollTimeout, periodic.TelemetryPollInterval).Should(Succeed())
		})
	})
})
