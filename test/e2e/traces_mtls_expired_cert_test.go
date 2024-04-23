//go:build e2e

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/tlsgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Traces mTLS with expired certificate", Label("traces"), func() {
	const (
		mockBackendName = "traces-tls-receiver"
		mockNs          = "traces-mocks-expired-tls"
		telemetrygenNs  = "trace-expired-mtls-cert"
	)
	var (
		pipelineName string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject(),
			kitk8s.NewNamespace(telemetrygenNs).K8sObject(),
		)

		serverCerts, clientCerts, err := tlsgen.NewCertBuilder(mockBackendName, mockNs).
			WithExpiredClientCert().
			Build()
		Expect(err).ToNot(HaveOccurred())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeTraces, backend.WithTLS(*serverCerts))
		objs = append(objs, mockBackend.K8sObjects()...)

		tracePipeline := kitk8s.NewTracePipelineV1Alpha1(fmt.Sprintf("%s-%s", mockBackend.Name(), "pipeline")).
			WithOutputEndpointFromSecret(mockBackend.HostSecretRefV1Alpha1()).
			WithTLS(*clientCerts)
		pipelineName = tracePipeline.Name()

		objs = append(objs,
			telemetrygen.New(telemetrygenNs, telemetrygen.SignalTypeTraces).K8sObject(),
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
			verifiers.TracePipelineShouldNotBeHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have a tlsCertificateExpired Condition set in pipeline conditions", func() {
			verifiers.TracePipelineShouldHaveTLSCondition(ctx, k8sClient, pipelineName, conditions.ReasonTLSCertificateExpired)
		})

		It("Should have telemetryCR showing tls certificate expired for trace component in its status", func() {
			verifiers.TelemetryShouldHaveCondition(ctx, k8sClient, "TraceComponentsHealthy", conditions.ReasonTLSCertificateExpired, false)
		})
	})
})
