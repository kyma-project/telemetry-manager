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

var _ = Describe(suite.ID(), Label(suite.LabelTraces), func() {
	const (
		missingValuesValidationError = "Can define either 'cert', 'key', and optionally 'ca', or 'ca' only"
		notFoundError                = "not found"
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

		backend := backend.New(mockNs, backend.SignalTypeTraces, backend.WithTLS(*serverCerts))
		succeedingObjs = append(succeedingObjs, backend.K8sObjects()...)

		tracePipelineMissingCa := testutils.NewTracePipelineBuilder().
			WithName(missingCaPipelineName).
			WithOTLPOutput(
				testutils.OTLPEndpoint(backend.Endpoint()),
				testutils.OTLPClientTLSMissingCA(
					clientCerts.ClientCertPem.String(),
					clientCerts.ClientKeyPem.String(),
				),
			).
			Build()

		tracePipelineMissingCert := testutils.NewTracePipelineBuilder().
			WithName(missingCertPipelineName).
			WithOTLPOutput(
				testutils.OTLPEndpoint(backend.Endpoint()),
				testutils.OTLPClientTLSMissingCert(
					clientCerts.CaCertPem.String(),
					clientCerts.ClientKeyPem.String(),
				),
			).
			Build()

		tracePipelineMissingKey := testutils.NewTracePipelineBuilder().
			WithName(missingKeyPipelineName).
			WithOTLPOutput(
				testutils.OTLPEndpoint(backend.Endpoint()),
				testutils.OTLPClientTLSMissingKey(
					clientCerts.CaCertPem.String(),
					clientCerts.ClientCertPem.String(),
				),
			).
			Build()

		tracePipelineMissingAll := testutils.NewTracePipelineBuilder().
			WithName(missingAllPipelineName).
			WithOTLPOutput(
				testutils.OTLPEndpoint(backend.Endpoint()),
				testutils.OTLPClientTLSMissingAll(),
			).
			Build()

		tracePipelineMissingAllButCa := testutils.NewTracePipelineBuilder().
			WithName(missingAllButCaPipelineName).
			WithOTLPOutput(
				testutils.OTLPEndpoint(backend.Endpoint()),
				testutils.OTLPClientTLSMissingAllButCA(
					clientCerts.CaCertPem.String(),
				),
			).
			Build()

		succeedingObjs = append(succeedingObjs,
			telemetrygen.NewPod(mockNs, telemetrygen.SignalTypeTraces).K8sObject(),
			&tracePipelineMissingCa, &tracePipelineMissingAllButCa,
		)

		failingObjs = append(failingObjs,
			&tracePipelineMissingKey, &tracePipelineMissingCert, &tracePipelineMissingAll,
		)

		return succeedingObjs, failingObjs
	}

	Context("When a trace pipeline with missing TLS configuration parameters is created", Ordered, func() {
		BeforeAll(func() {
			k8sSucceedingObjects, k8sFailingObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sSucceedingObjects...)).Should(Succeed())
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sFailingObjects...)).
					Should(MatchError(ContainSubstring(notFoundError)))
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sSucceedingObjects...)).Should(Succeed())
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sFailingObjects...)).
				Should(MatchError(ContainSubstring(missingValuesValidationError)))
		})

		It("Should set ConfigurationGenerated condition to True in pipelines", func() {
			assert.TracePipelineHasCondition(ctx, k8sClient, missingCaPipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonGatewayConfigured,
			})

			assert.TracePipelineHasCondition(ctx, k8sClient, missingAllButCaPipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonGatewayConfigured,
			})
		})

		It("Should set TelemetryFlowHealthy condition to True in pipelines", func() {
			assert.TracePipelineHasCondition(ctx, k8sClient, missingCaPipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonSelfMonFlowHealthy,
			})

			assert.TracePipelineHasCondition(ctx, k8sClient, missingAllButCaPipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonSelfMonFlowHealthy,
			})
		})

		It("Should set TraceComponentsHealthy condition to True in Telemetry", func() {
			assert.TelemetryHasReadyState(ctx, k8sClient)
			assert.TelemetryHasCondition(ctx, k8sClient, metav1.Condition{
				Type:   conditions.TypeTraceComponentsHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonComponentsRunning,
			})
		})
	})
})
