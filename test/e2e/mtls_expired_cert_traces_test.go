//go:build e2e

package e2e

import (
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Traces mTLS with expired certificate", Label("mtls"), func() {
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

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeMetrics, backend.WithTLS(time.Now(), time.Now().AddDate(0, 0, -7)))
		objs = append(objs, mockBackend.K8sObjects()...)
		//telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)

		tracePipeline := kitk8s.NewTracePipelineV1Alpha1(fmt.Sprintf("%s-%s", mockBackend.Name(), "pipeline")).
			WithOutputEndpointFromSecret(mockBackend.HostSecretRefV1Alpha1()).
			WithTLS(mockBackend.TLSCerts)
		pipelineName = tracePipeline.Name()

		objs = append(objs,
			telemetrygen.New(telemetrygenNs, telemetrygen.SignalTypeMetrics).K8sObject(),
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

		It("Should have a tls certificate expired Condition set in pipeline conditions", func() {
			verifiers.TracePipelineWithTLSCerExpiredCondition(ctx, k8sClient, pipelineName)
		})

		It("Should have telemetryCR showing tls certificate expired for trace component in its status", func() {
			verifiers.TelemetryCRShouldHaveTLSConditionForMetricPipeline(ctx, k8sClient, "TraceComponentsHealthy", conditions.ReasonTLSCertificateExpired, false)
		})

	})
})
