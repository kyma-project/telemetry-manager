//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	const (
		tlsCrdValidationError = "Can define either both 'cert' and 'key', or neither"
		notFoundError         = "not found"
	)

	var (
		mockNs                      = suite.ID()
		missingCaPipelineName       = suite.ID() + "-missing-ca"
		missingCertPipelineName     = suite.ID() + "-missing-cert"
		missingKeyPipelineName      = suite.ID() + "-missing-key"
		missingAllPipelineName      = suite.ID() + "-missing-all"
		missingAllButCaPipelineName = suite.ID() + "-missing-all-but-ca"
	)

	makeResources := func() ([]client.Object, []client.Object) {
		var succeedingObjs []client.Object
		var failingObjs []client.Object
		succeedingObjs = append(succeedingObjs, kitk8s.NewNamespace(mockNs).K8sObject())

		serverCerts, clientCerts, err := testutils.NewCertBuilder(backend.DefaultName, mockNs).Build()
		Expect(err).ToNot(HaveOccurred())

		backend := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithTLS(*serverCerts))
		succeedingObjs = append(succeedingObjs, backend.K8sObjects()...)

		metricPipelineMissingCa := testutils.NewMetricPipelineBuilder().
			WithName(missingCaPipelineName).
			WithOTLPOutput(
				testutils.OTLPEndpoint(backend.Endpoint()),
				testutils.OTLPClientTLSMissingCA(
					clientCerts.ClientCertPem.String(),
					clientCerts.ClientKeyPem.String(),
				),
			).
			Build()

		metricPipelineMissingCert := testutils.NewMetricPipelineBuilder().
			WithName(missingCertPipelineName).
			WithOTLPOutput(
				testutils.OTLPEndpoint(backend.Endpoint()),
				testutils.OTLPClientTLSMissingCert(
					clientCerts.CaCertPem.String(),
					clientCerts.ClientKeyPem.String(),
				),
			).
			Build()

		metricPipelineMissingKey := testutils.NewMetricPipelineBuilder().
			WithName(missingKeyPipelineName).
			WithOTLPOutput(
				testutils.OTLPEndpoint(backend.Endpoint()),
				testutils.OTLPClientTLSMissingKey(
					clientCerts.CaCertPem.String(),
					clientCerts.ClientCertPem.String(),
				),
			).
			Build()

		metricPipelineMissingAll := testutils.NewMetricPipelineBuilder().
			WithName(missingAllPipelineName).
			WithOTLPOutput(
				testutils.OTLPEndpoint(backend.Endpoint()),
				testutils.OTLPClientTLSMissingAll(),
			).
			Build()

		metricPipelineMissingAllButCa := testutils.NewMetricPipelineBuilder().
			WithName(missingAllButCaPipelineName).
			WithOTLPOutput(
				testutils.OTLPEndpoint(backend.Endpoint()),
				testutils.OTLPClientTLSMissingAllButCA(
					clientCerts.CaCertPem.String(),
				),
			).
			Build()

		succeedingObjs = append(succeedingObjs,
			telemetrygen.NewPod(mockNs, telemetrygen.SignalTypeMetrics).K8sObject(),
			&metricPipelineMissingCa, &metricPipelineMissingAllButCa, &metricPipelineMissingAll,
		)

		failingObjs = append(failingObjs,
			&metricPipelineMissingKey, &metricPipelineMissingCert,
		)

		return succeedingObjs, failingObjs
	}

	Context("When a metric pipeline with missing TLS configuration parameters is created", Ordered, func() {
		BeforeAll(func() {
			k8sSucceedingObjects, k8sFailingObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sSucceedingObjects...)).Should(Succeed())
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sFailingObjects...)).
					Should(MatchError(ContainSubstring(notFoundError)))
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sSucceedingObjects...)).Should(Succeed())
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sFailingObjects...)).
				Should(MatchError(ContainSubstring(tlsCrdValidationError)))
		})

		It("Should set ConfigurationGenerated condition to True in pipelines", func() {
			assert.MetricPipelineHasCondition(ctx, k8sClient, missingCaPipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonGatewayConfigured,
			})

			assert.MetricPipelineHasCondition(ctx, k8sClient, missingAllButCaPipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonGatewayConfigured,
			})

			assert.MetricPipelineHasCondition(ctx, k8sClient, missingAllPipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonGatewayConfigured,
			})
		})

		It("Should set TelemetryFlowHealthy condition to True in pipelines", func() {
			assert.MetricPipelineHasCondition(ctx, k8sClient, missingCaPipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonSelfMonFlowHealthy,
			})

			assert.MetricPipelineHasCondition(ctx, k8sClient, missingAllButCaPipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonSelfMonFlowHealthy,
			})

			assert.MetricPipelineHasCondition(ctx, k8sClient, missingAllPipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonSelfMonFlowHealthy,
			})
		})

		It("Should set MetricComponentsHealthy condition to True in Telemetry", func() {
			assert.TelemetryHasReadyState(ctx, k8sClient)
			assert.TelemetryHasCondition(ctx, k8sClient, metav1.Condition{
				Type:   conditions.TypeMetricComponentsHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonComponentsRunning,
			})
		})
	})
})
