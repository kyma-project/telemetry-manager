//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/tlsgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogs), Ordered, func() {
	var (
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		serverCerts, clientCerts, err := tlsgen.NewCertBuilder(backend.DefaultName, mockNs).
			WithAboutToExpireClientCert().
			Build()
		Expect(err).ToNot(HaveOccurred())

		backend := backend.New(mockNs, backend.SignalTypeLogs, backend.WithTLS(*serverCerts))
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(proxyClient)

		logPipeline := kitk8s.NewLogPipelineV1Alpha1(pipelineName).
			WithSecretKeyRef(backend.HostSecretRefV1Alpha1()).
			WithHTTPOutput().
			WithTLS(*clientCerts)

		logProducer := loggen.New(mockNs)
		objs = append(objs, logProducer.K8sObject())

		objs = append(objs,
			logPipeline.K8sObject(),
		)

		return objs
	}

	Context("When a log pipeline with TLS Cert expiring within 2 weeks is activated", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			verifiers.LogPipelineShouldBeHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have a tlsCertAboutToExpire Condition set in pipeline conditions", func() {
			verifiers.LogPipelineShouldHaveTLSCondition(ctx, k8sClient, pipelineName, conditions.ReasonTLSCertificateAboutToExpire)
		})

		It("Should have telemetryCR showing correct condition in its status", func() {
			verifiers.TelemetryShouldHaveCondition(ctx, k8sClient, "LogComponentsHealthy", conditions.ReasonTLSCertificateAboutToExpire, true)
		})

		It("Should have a log backend running", Label(suite.LabelOperational), func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: backend.DefaultName})
		})

		It("Should have a log producer running", Label(suite.LabelOperational), func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: loggen.DefaultName})
		})

		It("Should have produced logs in the backend", Label(suite.LabelOperational), func() {
			verifiers.LogsShouldBeDelivered(proxyClient, loggen.DefaultName, backendExportURL)
		})
	})
})
