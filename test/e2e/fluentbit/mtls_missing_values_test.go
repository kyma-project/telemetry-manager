//go:build e2e

package fluentbit

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelLogs), Ordered, func() {
	const (
		tlsCrdValidationError = "Can define either both 'cert' and 'key', or neither"
		notFoundError         = "not found"
	)

	var (
		mockNs                      = ID()
		missingCaPipelineName       = IDWithSuffix("-missing-ca")
		missingCertPipelineName     = IDWithSuffix("-missing-cert")
		missingKeyPipelineName      = IDWithSuffix("-missing-key")
		missingAllPipelineName      = IDWithSuffix("-missing-all")
		missingAllButCaPipelineName = IDWithSuffix("-missing-all-but-ca")
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
				testutils.HTTPClientTLS(telemetryv1alpha1.LogPipelineOutputTLS{
					Cert: &telemetryv1alpha1.ValueType{Value: clientCerts.ClientCertPem.String()},
					Key:  &telemetryv1alpha1.ValueType{Value: clientCerts.ClientKeyPem.String()},
				}),
			).
			Build()

		logPipelineMissingCert := testutils.NewLogPipelineBuilder().
			WithName(missingCertPipelineName).
			WithHTTPOutput(
				testutils.HTTPHost(backend.Host()),
				testutils.HTTPPort(backend.Port()),
				testutils.HTTPClientTLS(telemetryv1alpha1.LogPipelineOutputTLS{
					CA:  &telemetryv1alpha1.ValueType{Value: clientCerts.CaCertPem.String()},
					Key: &telemetryv1alpha1.ValueType{Value: clientCerts.ClientKeyPem.String()},
				}),
			).
			Build()

		logPipelineMissingKey := testutils.NewLogPipelineBuilder().
			WithName(missingKeyPipelineName).
			WithHTTPOutput(
				testutils.HTTPHost(backend.Host()),
				testutils.HTTPPort(backend.Port()),
				testutils.HTTPClientTLS(telemetryv1alpha1.LogPipelineOutputTLS{
					CA:   &telemetryv1alpha1.ValueType{Value: clientCerts.CaCertPem.String()},
					Cert: &telemetryv1alpha1.ValueType{Value: clientCerts.ClientCertPem.String()},
				}),
			).
			Build()

		logPipelineMissingAll := testutils.NewLogPipelineBuilder().
			WithName(missingAllPipelineName).
			WithHTTPOutput(
				testutils.HTTPHost(backend.Host()),
				testutils.HTTPPort(backend.Port()),
				testutils.HTTPClientTLS(telemetryv1alpha1.LogPipelineOutputTLS{
					Disabled:                  true,
					SkipCertificateValidation: true,
				}),
			).
			Build()

		logPipelineMissingAllButCa := testutils.NewLogPipelineBuilder().
			WithName(missingAllButCaPipelineName).
			WithHTTPOutput(
				testutils.HTTPHost(backend.Host()),
				testutils.HTTPPort(backend.Port()),
				testutils.HTTPClientTLS(telemetryv1alpha1.LogPipelineOutputTLS{
					CA: &telemetryv1alpha1.ValueType{Value: clientCerts.CaCertPem.String()},
				}),
			).
			Build()

		logProducer := loggen.New(mockNs)
		succeedingObjs = append(succeedingObjs, logProducer.K8sObject())

		succeedingObjs = append(succeedingObjs,
			&logPipelineMissingCa, &logPipelineMissingAllButCa, &logPipelineMissingAll,
		)

		failingObjs = append(failingObjs,
			&logPipelineMissingKey, &logPipelineMissingCert,
		)

		return succeedingObjs, failingObjs
	}

	Context("When a log pipeline with missing TLS configuration parameters is created", Ordered, func() {
		BeforeAll(func() {
			k8sSucceedingObjects, k8sFailingObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, k8sSucceedingObjects...)).Should(Succeed())
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, k8sFailingObjects...)).
					Should(MatchError(ContainSubstring(notFoundError)))
			})
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, k8sSucceedingObjects...)).Should(Succeed())
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, k8sFailingObjects...)).
				Should(MatchError(ContainSubstring(tlsCrdValidationError)))
		})

		It("Should set ConfigurationGenerated condition to True in pipelines", func() {
			assert.LogPipelineHasCondition(Ctx, K8sClient, missingCaPipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonAgentConfigured,
			})

			assert.LogPipelineHasCondition(Ctx, K8sClient, missingAllButCaPipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonAgentConfigured,
			})

			assert.LogPipelineHasCondition(Ctx, K8sClient, missingAllPipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonAgentConfigured,
			})
		})

		It("Should set TelemetryFlowHealthy condition to True in pipelines", func() {
			assert.LogPipelineHasCondition(Ctx, K8sClient, missingCaPipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonSelfMonFlowHealthy,
			})

			assert.LogPipelineHasCondition(Ctx, K8sClient, missingAllButCaPipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonSelfMonFlowHealthy,
			})

			assert.LogPipelineHasCondition(Ctx, K8sClient, missingAllPipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonSelfMonFlowHealthy,
			})
		})

		It("Should set LogComponentsHealthy condition to True in Telemetry", func() {
			assert.TelemetryHasState(Ctx, K8sClient, operatorv1alpha1.StateReady)
			assert.TelemetryHasCondition(Ctx, K8sClient, metav1.Condition{
				Type:   conditions.TypeLogComponentsHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonComponentsRunning,
			})
		})
	})
})
