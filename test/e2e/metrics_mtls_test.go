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
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
	kitmetrics "github.com/kyma-project/telemetry-manager/test/testkit/otel/metrics"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics mTLS", Label("metrics"), func() {
	const (
		mockBackendName = "metric-tls-receiver"
		mockNs          = "metric-mocks-tls-pipeline"
	)
	var (
		pipelineName string
		urls         = urlprovider.New()
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeMetrics, backend.WithTLS())
		objs = append(objs, mockBackend.K8sObjects()...)
		urls.SetMockBackendExport(mockBackend.Name(), mockBackend.TelemetryExportURL(proxyClient))

		metricPipeline := kitk8s.NewMetricPipeline(fmt.Sprintf("%s-%s", mockBackend.Name(), "pipeline")).
			WithOutputEndpointFromSecret(mockBackend.HostSecretRef()).
			WithTLS(mockBackend.TLSCerts)
		pipelineName = metricPipeline.Name()

		objs = append(objs, metricPipeline.K8sObject())
		urls.SetOTLPPush(proxyClient.ProxyURLForService(
			kitkyma.SystemNamespaceName, "telemetry-otlp-metrics", "v1/metrics/", ports.OTLPHTTP),
		)

		return objs
	}

	Context("When a metricpipeline with TLS activated exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have a metric backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName, Namespace: mockNs})
		})

		It("Should verify end-to-end metric delivery", func() {
			gauges := kitmetrics.MakeAndSendGaugeMetrics(proxyClient, urls.OTLPPush())
			verifiers.MetricsShouldBeDelivered(proxyClient, urls.MockBackendExport(mockBackendName), gauges)
		})
	})
})
