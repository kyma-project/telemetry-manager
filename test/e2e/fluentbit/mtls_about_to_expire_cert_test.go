//go:build e2e

package fluentbit

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelLogs), Ordered, func() {
	var (
		mockNs           = ID()
		pipelineName     = ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		serverCerts, clientCerts, err := testutils.NewCertBuilder(backend.DefaultName, mockNs).
			WithAboutToExpireClientCert().
			Build()
		Expect(err).ToNot(HaveOccurred())

		backend := backend.New(mockNs, backend.SignalTypeLogs, backend.WithTLS(*serverCerts))
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(ProxyClient)

		logPipeline := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithHTTPOutput(
				testutils.HTTPHost(backend.Host()),
				testutils.HTTPPort(backend.Port()),
				testutils.HTTPClientTLSFromString(
					clientCerts.CaCertPem.String(),
					clientCerts.ClientCertPem.String(),
					clientCerts.ClientKeyPem.String(),
				)).
			Build()

		logProducer := loggen.New(mockNs)
		objs = append(objs, logProducer.K8sObject())

		objs = append(objs, &logPipeline)
		return objs
	}

	Context("When a log pipeline with TLS Cert expiring within 2 weeks is activated", Ordered, func() {
		BeforeAll(func() {
			K8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			assert.LogPipelineHealthy(Ctx, K8sClient, pipelineName)
		})

		It("Should have running log agent", func() {
			assert.DaemonSetReady(Ctx, K8sClient, kitkyma.FluentBitDaemonSetName)
		})

		It("Should have a tlsCertAboutToExpire Condition set in pipeline conditions", func() {
			assert.LogPipelineHasCondition(Ctx, K8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonTLSCertificateAboutToExpire,
			})
		})

		It("Should have telemetryCR showing correct condition in its status", func() {
			assert.TelemetryHasState(Ctx, K8sClient, operatorv1alpha1.StateWarning)
			assert.TelemetryHasCondition(Ctx, K8sClient, metav1.Condition{
				Type:   conditions.TypeLogComponentsHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonTLSCertificateAboutToExpire,
			})
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Namespace: mockNs, Name: backend.DefaultName})
		})

		It("Should have a log producer running", func() {
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Namespace: mockNs, Name: loggen.DefaultName})
		})

		It("Should have produced logs in the backend", func() {
			assert.LogsDelivered(ProxyClient, loggen.DefaultName, backendExportURL)
		})
	})
})
