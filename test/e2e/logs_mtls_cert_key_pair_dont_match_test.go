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

		logPipeline := kitk8s.NewLogPipelineV1Alpha1(pipelineName).
			WithSecretKeyRef(backend.HostSecretRefV1Alpha1()).
			WithHTTPOutput().
			WithTLS(*invalidClientCerts)
		pipelineName = logPipeline.Name()

		logProducer := loggen.New(mockNs)
		objs = append(objs, logProducer.K8sObject())

		objs = append(objs,
			logPipeline.K8sObject(),
		)

		return objs
	}

	Context("When a log pipeline is configured TLS Cert that does not match the Key", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should not have running pipelines", func() {
			assert.LogPipelineNotHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have a tls certificate key pair invalid condition set in pipeline conditions", func() {
			assert.LogPipelineHasCondition(ctx, k8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonTLSCertificateKeyPairInvalid,
			})
		})

		It("Should have telemetryCR showing tls certificate key pair invalid condition for log component in its status", func() {
			assert.TelemetryHasWarningState(ctx, k8sClient)
			assert.TelemetryHasCondition(ctx, k8sClient, metav1.Condition{
				Type:   "LogComponentsHealthy",
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonTLSCertificateKeyPairInvalid,
			})
		})
	})
})
