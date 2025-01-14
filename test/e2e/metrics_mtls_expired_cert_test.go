//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Label(suite.LabelSetB), Ordered, func() {
	var (
		mockNs       = suite.ID()
		pipelineName = suite.ID()
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		serverCerts, clientCerts, err := testutils.NewCertBuilder(backend.DefaultName, mockNs).
			WithExpiredClientCert().
			Build()
		Expect(err).ToNot(HaveOccurred())

		backend := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithTLS(*serverCerts))
		objs = append(objs, backend.K8sObjects()...)

		metricPipeline := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(
				testutils.OTLPEndpoint(backend.Endpoint()),
				testutils.OTLPClientTLSFromString(
					clientCerts.CaCertPem.String(),
					clientCerts.ClientCertPem.String(),
					clientCerts.ClientKeyPem.String(),
				),
			).
			Build()

		objs = append(objs,
			telemetrygen.NewPod(mockNs, telemetrygen.SignalTypeMetrics).K8sObject(),
			&metricPipeline,
		)

		return objs
	}

	Context("When a metric pipeline with an expired TLS Cert is created", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should set ConfigurationGenerated condition to False in pipeline", func() {
			assert.MetricPipelineHasCondition(ctx, k8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonTLSCertificateExpired,
			})
		})

		It("Should set TelemetryFlowHealthy condition to False in pipeline", func() {
			assert.MetricPipelineHasCondition(ctx, k8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonConfigNotGenerated,
			})
		})

		It("Should set MetricComponentsHealthy condition to False in Telemetry", func() {
			assert.TelemetryHasState(ctx, k8sClient, operatorv1alpha1.StateWarning)
			assert.TelemetryHasCondition(ctx, k8sClient, metav1.Condition{
				Type:   conditions.TypeMetricComponentsHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonTLSCertificateExpired,
			})
		})

	})
})
