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

var _ = Describe("Metrics mTLS with invalid certificate", Label("metrics"), func() {
	const (
		mockBackendName = "metrics-tls-receiver"
		mockNs          = "metrics-mocks-invalid-tls"
		telemetrygenNs  = "metrics-invalid-mtls-cert"
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
		buf := bytes.Buffer{}
		buf.WriteString("invalid cert")

		certs := tls.Certs{
			CaCertPem:     buf,
			ServerCertPem: buf,
			ServerKeyPem:  buf,
			ClientCertPem: buf,
			ClientKeyPem:  buf,
		}

		metricPipeline := kitk8s.NewMetricPipelineV1Alpha1(fmt.Sprintf("%s-%s", mockBackend.Name(), "pipeline")).
			WithOutputEndpointFromSecret(mockBackend.HostSecretRefV1Alpha1()).
			WithTLS(certs)
		pipelineName = metricPipeline.Name()

		objs = append(objs,
			telemetrygen.New(telemetrygenNs, telemetrygen.SignalTypeMetrics).K8sObject(),
			metricPipeline.K8sObject(),
		)

		return objs
	}

	Context("When a metric pipeline with invalid TLS Cert is created", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should not have running pipelines", func() {
			verifiers.MetricPipelineShouldNotBeHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have a tls certificate invalid Condition set in pipeline conditions", func() {
			verifiers.MetricPipelineWithTLSCertCondition(ctx, k8sClient, pipelineName, conditions.ReasonTLSCertificateInvalid)
		})

		It("Should have telemetryCR showing tls certificate expired for metric component in its status", func() {
			verifiers.TelemetryCRShouldHaveTLSConditionForPipeline(ctx, k8sClient, "MetricComponentsHealthy", conditions.ReasonTLSCertificateInvalid, false)
		})

	})
})
