//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

		backend := backend.New(mockNs, backend.SignalTypeTraces, backend.WithTLS(*serverCerts))
		objs = append(objs, backend.K8sObjects()...)

		tracePipeline := kitk8s.NewTracePipelineV1Alpha1(pipelineName).
			WithOutputEndpointFromSecret(backend.HostSecretRefV1Alpha1()).
			WithTLS(*clientCerts)

		objs = append(objs,
			telemetrygen.New(mockNs, telemetrygen.SignalTypeTraces).K8sObject(),
			tracePipeline.K8sObject(),
		)

		return objs
	}

	Context("When a trace pipeline with TLS Cert is expired", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should not have running pipelines", func() {
			assert.TracePipelineNotHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have a tlsCertificateExpired Condition set in pipeline conditions", func() {
			assert.TracePipelineHasCondition(ctx, k8sClient, pipelineName, conditions.ReasonTLSCertificateExpired)
		})

		It("Should have telemetryCR showing tls certificate expired for trace component in its status", func() {
			assert.TelemetryHasWarningState(ctx, k8sClient)
			assert.TelemetryHasCondition(ctx, k8sClient, "TraceComponentsHealthy", conditions.ReasonTLSCertificateExpired, false)
		})
	})
})
