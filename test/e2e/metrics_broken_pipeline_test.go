//go:build e2e

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitmetric "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
	kitmetrics "github.com/kyma-project/telemetry-manager/test/testkit/otlp/metrics"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics", Label("metrics"), func() {
	const (
		mockBackendName    = "metric-receiver"
		mockNs             = "metric-mocks-broken-pipeline"
		brokenPipelineName = "broken-metric-pipeline"
	)
	var (
		pipelineName string
		urls         *urlprovider.URLProvider
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeMetrics)
		objs = append(objs, mockBackend.K8sObjects()...)
		urls.SetMockBackendExport(mockBackend.Name(), mockBackend.TelemetryExportURL(proxyClient))

		metricPipeline := kitmetric.NewPipeline(fmt.Sprintf("%s-%s", mockBackend.Name(), "pipeline"), mockBackend.HostSecretRefKey())
		pipelineName = metricPipeline.Name()
		objs = append(objs, metricPipeline.K8sObject())

		hostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname-"+brokenPipelineName, defaultNamespaceName, kitk8s.WithStringData("metric-host", "http://unreachable:4317"))
		brokenMetricPipeline := kitmetric.NewPipeline(brokenPipelineName, hostSecret.SecretKeyRef("metric-host"))
		brokenPipelineObjs := []client.Object{hostSecret.K8sObject(), brokenMetricPipeline.K8sObject()}
		objs = append(objs, brokenPipelineObjs...)

		urls.SetOTLPPush(proxyClient.ProxyURLForService(
			kymaSystemNamespaceName, "telemetry-otlp-metrics", "v1/metrics/", ports.OTLPHTTP),
		)

		return objs
	}
	Context("When a broken metricpipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			verifiers.MetricPipelineShouldBeRunning(ctx, k8sClient, pipelineName)
			verifiers.MetricPipelineShouldBeRunning(ctx, k8sClient, brokenPipelineName)
		})

		It("Should have a running metric gateway deployment", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, metricGatewayName)

		})

		It("Should have a metrics backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName, Namespace: mockNs})
		})

		It("Should verify end-to-end metric delivery", func() {
			gauges := kitmetrics.MakeAndSendGaugeMetrics(proxyClient, urls.OTLPPush())
			verifiers.MetricsShouldBeDelivered(proxyClient, urls.MockBackendExport(mockBackendName), gauges)
		})
	})

})
