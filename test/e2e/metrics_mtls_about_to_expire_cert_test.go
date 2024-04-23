//go:build e2e

package e2e

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics mTLS with certificates expiring within 2 weeks", Label("metrics"), func() {
	const (
		mockBackendName = "metric-tls-receiver"
		mockNs          = "metric-mocks-2week-tls-pipeline"
		telemetrygenNs  = "metric-mtls-2weeks-to-expire"
	)
	var (
		pipelineName       string
		telemetryExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject(),
			kitk8s.NewNamespace(telemetrygenNs).K8sObject(),
		)

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeMetrics, backend.WithTLS(time.Now(), time.Now().AddDate(0, 0, 7)))
		objs = append(objs, mockBackend.K8sObjects()...)
		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)

		metricPipeline := kitk8s.NewMetricPipelineV1Alpha1(fmt.Sprintf("%s-%s", mockBackend.Name(), "pipeline")).
			WithOutputEndpointFromSecret(mockBackend.HostSecretRefV1Alpha1()).
			WithTLS(mockBackend.TLSCerts)
		pipelineName = metricPipeline.Name()

		objs = append(objs,
			telemetrygen.New(telemetrygenNs, telemetrygen.SignalTypeMetrics).K8sObject(),
			metricPipeline.K8sObject(),
		)

		return objs
	}

	Context("When a metric pipeline with TLS Cert expiring within 2 weeks is activated", Ordered, func() {
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

		It("Should have a tlsCertAboutToExpire Condition set in pipeline conditions", func() {
			verifiers.MetricPipelineWithTLSCertCondition(ctx, k8sClient, pipelineName, conditions.ReasonTLSCertificateAboutToExpire)
		})

		It("Should have telemetryCR showing correct condition in its status", func() {
			verifiers.TelemetryCRShouldHaveTLSConditionForPipeline(ctx, k8sClient, "MetricComponentsHealthy", conditions.ReasonTLSCertificateAboutToExpire, true)
		})

		It("Should have a metric backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName, Namespace: mockNs})
		})

		It("Should deliver telemetrygen metrics", func() {
			verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, telemetryExportURL, telemetrygenNs, telemetrygen.MetricNames)
		})
	})
})
