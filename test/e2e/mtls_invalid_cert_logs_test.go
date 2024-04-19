//go:build e2e

package e2e

import (
	"bytes"
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend/tls"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Logs mTLS with invalid certificate", Label("mtls"), func() {
	const (
		mockBackendName = "logs-tls-receiver"
		mockNs          = "logs-mocks-invalid-tls"
		telemetrygenNs  = "logs-invalid-mtls-cert"
	)
	var (
		pipelineName string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject(),
			kitk8s.NewNamespace(telemetrygenNs).K8sObject(),
		)

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeMetrics, backend.WithTLS(time.Now(), time.Now().AddDate(0, 0, 7)))
		objs = append(objs, mockBackend.K8sObjects()...)
		//telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)
		buf := bytes.Buffer{}
		buf.WriteString("invalid cert")

		certs := tls.Certs{
			CaCertPem:     buf,
			ServerCertPem: buf,
			ServerKeyPem:  buf,
			ClientCertPem: buf,
			ClientKeyPem:  buf,
		}

		logPipeline := kitk8s.NewLogPipelineV1Alpha1(fmt.Sprintf("%s-%s", mockBackend.Name(), "pipeline")).
			WithSecretKeyRef(mockBackend.HostSecretRefV1Alpha1()).
			WithHTTPOutput().
			WithTLS(certs)
		pipelineName = logPipeline.Name()

		objs = append(objs,
			telemetrygen.New(telemetrygenNs, telemetrygen.SignalTypeMetrics).K8sObject(),
			logPipeline.K8sObject(),
		)

		return objs
	}

	Context("When a log pipeline with invalid TLS Cert is created", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should not have running pipelines", func() {
			verifiers.LogPipelineShouldNotBeHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have a tls certificate expired Condition set in pipeline conditions", func() {
			verifiers.LogPipelineWithInvalidTLSCertCondition(ctx, k8sClient, pipelineName)
		})

		It("Should have telemetryCR showing tls certificate expired for log component in its status", func() {
			verifiers.TelemetryCRShouldHaveTLSConditionForMetricPipeline(ctx, k8sClient, "LogComponentsHealthy", conditions.ReasonTLSCertificateInvalid, false)
		})

	})
})
