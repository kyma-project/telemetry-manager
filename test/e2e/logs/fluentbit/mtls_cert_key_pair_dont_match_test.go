//go:build e2e

package fluentbit

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogsFluentBit), Ordered, func() {
	var (
		mockNs       = suite.ID()
		pipelineName = suite.ID()
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		serverCertsDefault, clientCertsDefault, err := testutils.NewCertBuilder(backend.DefaultName, mockNs).Build()
		Expect(err).ToNot(HaveOccurred())

		_, clientCertsCreatedAgain, err := testutils.NewCertBuilder(backend.DefaultName, mockNs).Build()
		Expect(err).ToNot(HaveOccurred())

		backend := backend.New(mockNs, backend.SignalTypeLogs, backend.WithTLS(*serverCertsDefault))
		objs = append(objs, backend.K8sObjects()...)

		invalidClientCerts := &testutils.ClientCerts{
			CaCertPem:     clientCertsDefault.CaCertPem,
			ClientCertPem: clientCertsDefault.ClientCertPem,
			ClientKeyPem:  clientCertsCreatedAgain.ClientKeyPem,
		}

		logPipeline := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithHTTPOutput(
				testutils.HTTPHost(backend.Host()),
				testutils.HTTPPort(backend.Port()),
				testutils.HTTPClientTLSFromString(
					invalidClientCerts.CaCertPem.String(),
					invalidClientCerts.ClientCertPem.String(),
					invalidClientCerts.ClientKeyPem.String(),
				)).
			Build()

		logProducer := loggen.New(mockNs)
		objs = append(objs, logProducer.K8sObject())

		objs = append(objs, &logPipeline)

		return objs
	}

	Context("When a log pipeline with TLS Cert that does not match the Key is created", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should set ConfigurationGenerated condition to False in pipeline", func() {
			assert.LogPipelineHasCondition(suite.Ctx, suite.K8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonTLSConfigurationInvalid,
			})
		})

		It("Should set TelemetryFlowHealthy condition to False in pipeline", func() {
			assert.LogPipelineHasCondition(suite.Ctx, suite.K8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonConfigNotGenerated,
			})
		})

		It("Should set LogComponentsHealthy condition to False in Telemetry", func() {
			assert.TelemetryHasState(suite.Ctx, suite.K8sClient, operatorv1alpha1.StateWarning)
			assert.TelemetryHasCondition(suite.Ctx, suite.K8sClient, metav1.Condition{
				Type:   conditions.TypeLogComponentsHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonTLSConfigurationInvalid,
			})
		})
	})
})
