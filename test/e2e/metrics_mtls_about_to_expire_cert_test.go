//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Ordered, func() {
	var (
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		serverCerts, clientCerts, err := testutils.NewCertBuilder(backend.DefaultName, mockNs).
			WithAboutToExpireClientCert().
			Build()
		Expect(err).ToNot(HaveOccurred())

		backend := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithTLS(*serverCerts))
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(proxyClient)

		metricPipeline := kitk8s.NewMetricPipelineV1Alpha1(pipelineName).
			WithOutputEndpointFromSecret(backend.HostSecretRefV1Alpha1()).
			WithTLS(*clientCerts)

		objs = append(objs,
			telemetrygen.New(mockNs, telemetrygen.SignalTypeMetrics).K8sObject(),
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
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have a tlsCertAboutToExpire Condition set in pipeline conditions", func() {
			assert.MetricPipelineHasCondition(ctx, k8sClient, pipelineName, conditions.TypeConfigurationGenerated, conditions.ReasonTLSCertificateAboutToExpire)
		})

		It("Should have telemetryCR showing correct condition in its status", func() {
			assert.TelemetryHasWarningState(ctx, k8sClient)
			assert.TelemetryHasCondition(ctx, k8sClient, "MetricComponentsHealthy", conditions.ReasonTLSCertificateAboutToExpire, true)
		})

		It("Should have a metric backend running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should deliver telemetrygen metrics", func() {
			assert.MetricsFromNamespaceDelivered(proxyClient, backendExportURL, mockNs, telemetrygen.MetricNames)
		})
	})
})
