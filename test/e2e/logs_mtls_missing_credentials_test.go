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
	var (
		mockNs                  = suite.ID()
		missingCaPipelineName   = suite.ID() + "-missing-ca"
		missingCertPipelineName = suite.ID() + "-missing-cert"
		missingKeyPipelineName  = suite.ID() + "-missing-key"
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		serverCerts, clientCerts, err := testutils.NewCertBuilder(backend.DefaultName, mockNs).Build()
		Expect(err).ToNot(HaveOccurred())

		backend := backend.New(mockNs, backend.SignalTypeLogs, backend.WithTLS(*serverCerts))
		objs = append(objs, backend.K8sObjects()...)

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

		logProducer := loggen.New(mockNs)
		objs = append(objs, logProducer.K8sObject())
		objs = append(objs, &logPipelineMissingCa, &logPipelineMissingCert, &logPipelineMissingKey)

		return objs
	}

	Context("When a log pipeline with missing TLS configuration parameters is created", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should set ConfigurationGenerated condition accordingly in pipelines", func() {
			assert.LogPipelineHasCondition(ctx, k8sClient, missingCaPipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonAgentConfigured,
			})

			assert.LogPipelineHasCondition(ctx, k8sClient, missingCertPipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonTLSConfigurationInvalid,
			})

			assert.LogPipelineHasCondition(ctx, k8sClient, missingKeyPipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonTLSConfigurationInvalid,
			})
		})

		It("Should set Running/Pending condition accordingly in pipelines", func() {
			assert.LogPipelineHasCondition(ctx, k8sClient, missingCaPipelineName, metav1.Condition{
				Type:   conditions.TypeRunning,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonFluentBitDSReady,
			})

			assert.LogPipelineHasCondition(ctx, k8sClient, missingCertPipelineName, metav1.Condition{
				Type:   conditions.TypePending,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonTLSConfigurationInvalid,
			})

			assert.LogPipelineHasCondition(ctx, k8sClient, missingKeyPipelineName, metav1.Condition{
				Type:   conditions.TypePending,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonTLSConfigurationInvalid,
			})
		})

		It("Should set Pending/Running condition accordingly in pipelines", func() {
			assert.LogPipelineHasCondition(ctx, k8sClient, missingCaPipelineName, metav1.Condition{
				Type:   conditions.TypeRunning,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonFluentBitDSReady,
			})

			assert.LogPipelineHasCondition(ctx, k8sClient, missingCertPipelineName, metav1.Condition{
				Type:   conditions.TypePending,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonTLSConfigurationInvalid,
			})

			assert.LogPipelineHasCondition(ctx, k8sClient, missingKeyPipelineName, metav1.Condition{
				Type:   conditions.TypePending,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonTLSConfigurationInvalid,
			})
		})

		It("Should set TelemetryFlowHealthy condition accordingly in pipelines", func() {
			assert.LogPipelineHasCondition(ctx, k8sClient, missingCaPipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonSelfMonFlowHealthy,
			})

			assert.LogPipelineHasCondition(ctx, k8sClient, missingCertPipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonConfigNotGenerated,
			})

			assert.LogPipelineHasCondition(ctx, k8sClient, missingKeyPipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonConfigNotGenerated,
			})
		})

		It("Should set LogComponentsHealthy condition to False in Telemetry", func() {
			assert.TelemetryHasWarningState(ctx, k8sClient)
			assert.TelemetryHasCondition(ctx, k8sClient, metav1.Condition{
				Type:   conditions.TypeLogComponentsHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonTLSConfigurationInvalid,
			})
		})
	})
})
