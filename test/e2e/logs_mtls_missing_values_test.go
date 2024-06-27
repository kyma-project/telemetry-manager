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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogs), Ordered, func() {
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

		backend := backend.New(mockNs, backend.SignalTypeLogs, backend.WithTLS(*serverCerts))
		succeedingObjs = append(succeedingObjs, backend.K8sObjects()...)

		logPipelineMissingCa := testutils.NewLogPipelineBuilder().
			WithName(missingCaPipelineName).
			WithHTTPOutput(
				testutils.HTTPHost(backend.Host()),
				testutils.HTTPPort(backend.Port()),
				testutils.HTTPClientTLSMissingCA(
					clientCerts.ClientCertPem.String(),
					clientCerts.ClientKeyPem.String(),
				)).
			Build()

		logPipelineMissingCert := testutils.NewLogPipelineBuilder().
			WithName(missingCertPipelineName).
			WithHTTPOutput(
				testutils.HTTPHost(backend.Host()),
				testutils.HTTPPort(backend.Port()),
				testutils.HTTPClientTLSMissingCert(
					clientCerts.CaCertPem.String(),
					clientCerts.ClientKeyPem.String(),
				)).
			Build()

		logPipelineMissingKey := testutils.NewLogPipelineBuilder().
			WithName(missingKeyPipelineName).
			WithHTTPOutput(
				testutils.HTTPHost(backend.Host()),
				testutils.HTTPPort(backend.Port()),
				testutils.HTTPClientTLSMissingKey(
					clientCerts.CaCertPem.String(),
					clientCerts.ClientCertPem.String(),
				)).
			Build()

		logPipelineMissingAll := testutils.NewLogPipelineBuilder().
			WithName(missingAllPipelineName).
			WithHTTPOutput(
				testutils.HTTPHost(backend.Host()),
				testutils.HTTPPort(backend.Port()),
				testutils.HTTPClientTLSMissingAll(),
			).
			Build()

		logPipelineMissingAllButCa := testutils.NewLogPipelineBuilder().
			WithName(missingAllButCaPipelineName).
			WithHTTPOutput(
				testutils.HTTPHost(backend.Host()),
				testutils.HTTPPort(backend.Port()),
				testutils.HTTPClientTLSMissingAllButCA(
					clientCerts.CaCertPem.String(),
				),
			).
			Build()

		logProducer := loggen.New(mockNs)
		succeedingObjs = append(succeedingObjs, logProducer.K8sObject())

		succeedingObjs = append(succeedingObjs,
			&logPipelineMissingCa, &logPipelineMissingAllButCa,
		)

		failingObjs = append(failingObjs,
			&logPipelineMissingKey, &logPipelineMissingCert, &logPipelineMissingAll,
		)

		return succeedingObjs, failingObjs
	}

	Context("When a log pipeline with missing TLS configuration parameters is created", Ordered, func() {
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
			assert.LogPipelineHasCondition(ctx, k8sClient, missingCaPipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonAgentConfigured,
			})

			assert.LogPipelineHasCondition(ctx, k8sClient, missingAllButCaPipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonAgentConfigured,
			})
		})

		It("Should set Running condition to True in pipelines", func() {
			assert.LogPipelineHasCondition(ctx, k8sClient, missingCaPipelineName, metav1.Condition{
				Type:   conditions.TypeRunning,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonFluentBitDSReady,
			})

			assert.LogPipelineHasCondition(ctx, k8sClient, missingAllButCaPipelineName, metav1.Condition{
				Type:   conditions.TypeRunning,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonFluentBitDSReady,
			})
		})

		It("Should set Pending/Running condition to True in pipelines", func() {
			assert.LogPipelineHasCondition(ctx, k8sClient, missingCaPipelineName, metav1.Condition{
				Type:   conditions.TypeRunning,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonFluentBitDSReady,
			})

			assert.LogPipelineHasCondition(ctx, k8sClient, missingAllButCaPipelineName, metav1.Condition{
				Type:   conditions.TypeRunning,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonFluentBitDSReady,
			})
		})

		It("Should set TelemetryFlowHealthy condition to True in pipelines", func() {
			assert.LogPipelineHasCondition(ctx, k8sClient, missingCaPipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonSelfMonFlowHealthy,
			})

			assert.LogPipelineHasCondition(ctx, k8sClient, missingAllButCaPipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonSelfMonFlowHealthy,
			})
		})

		It("Should set LogComponentsHealthy condition to True in Telemetry", func() {
			assert.TelemetryHasReadyState(ctx, k8sClient)
			assert.TelemetryHasCondition(ctx, k8sClient, metav1.Condition{
				Type:   conditions.TypeLogComponentsHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonComponentsRunning,
			})
		})
	})
})