//go:build e2e

package selfmonitor

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

// Please remove the test when the compatibility mode annotation feature removed, planed for telemetry version 1.41.0
var _ = Describe(suite.ID(), Label(suite.LabelSelfMonitoringMetricsHealthy), Ordered, func() {
	var (
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := kitbackend.New(mockNs, kitbackend.SignalTypeMetrics)
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(suite.ProxyClient)

		pipeline := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()
		objs = append(objs,
			telemetrygen.NewPod(kitkyma.DefaultNamespaceName, telemetrygen.SignalTypeMetrics).K8sObject(),
			&pipeline,
		)

		return objs
	}

	Context("When a metric pipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
		})

		It("Should have global internal metrics compatibility annotation config", func() {
			Eventually(func(g Gomega) string {
				var telemetry operatorv1alpha1.Telemetry
				err := suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)
				g.Expect(err).NotTo(HaveOccurred())

				telemetry.Annotations = map[string]string{
					"telemetry.kyma-project.io/internal-metrics-compatibility-mode": "true",
				}

				err = suite.K8sClient.Update(suite.Ctx, &telemetry)
				g.Expect(err).NotTo(HaveOccurred())
				return telemetry.Annotations["telemetry.kyma-project.io/internal-metrics-compatibility-mode"]
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal("true"))
		})

		It("Should have a running self-monitor", func() {
			assert.DeploymentReady(suite.Ctx, kitkyma.SelfMonitorName)
		})

		It("Should have service deployed", func() {
			var service corev1.Service
			Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.SelfMonitorName, &service)).To(Succeed())
		})

		It("Should have a metrics backend running", func() {
			assert.DeploymentReady(suite.Ctx, types.NamespacedName{Name: kitbackend.DefaultName, Namespace: mockNs})
			assert.ServiceReady(suite.Ctx, types.NamespacedName{Name: kitbackend.DefaultName, Namespace: mockNs})
		})

		It("Should have a running pipeline", func() {
			assert.MetricPipelineHealthy(suite.Ctx, pipelineName)
		})

		It("Ensures the metric gateway deployment is ready", func() {
			assert.DeploymentReady(suite.Ctx, kitkyma.MetricGatewayName)
		})

		It("Should deliver telemetrygen metrics", func() {
			assert.MetricsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, kitkyma.DefaultNamespaceName, telemetrygen.MetricNames)
		})

		It("Should have TypeFlowHealthy condition set to True", func() {
			Eventually(func(g Gomega) {
				var pipeline telemetryv1alpha1.MetricPipeline
				key := types.NamespacedName{Name: pipelineName}
				g.Expect(suite.K8sClient.Get(suite.Ctx, key, &pipeline)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeFlowHealthy)).To(BeTrueBecause("Flow not healthy"))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})
	})
})
