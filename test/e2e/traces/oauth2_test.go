package traces

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	kitoauth2 "github.com/kyma-project/telemetry-manager/test/testkit/mocks/oauth2mock"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestSinglePipelineWithOAuth2(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTraces)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
	)

	oauth2server := kitoauth2.New(backendNs)

	serverCerts, _, err := testutils.NewCertBuilder(kitbackend.DefaultName, backendNs).Build()
	Expect(err).ToNot(HaveOccurred())

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces,
		kitbackend.WithTLS(*serverCerts),
		kitbackend.WithOIDCAuth(oauth2server.IssuerURL(), oauth2server.Audience()),
	)
	pipeline := testutils.NewTracePipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(
			testutils.OTLPEndpoint(backend.Endpoint()),
			testutils.OTLPOAuth2(
				testutils.WithOAuth2TokenURL(oauth2server.TokenEndpoint()),
				testutils.WithOAuth2ClientID("the-mock-does-not-verify"),
				testutils.WithOAuth2ClientSecret("the-mock-does-not-verify"),
				testutils.WithOAuth2Params(map[string]string{"grant_type": "client_credentials"}),
			),
			testutils.OTLPClientTLS(
				&telemetryv1alpha1.OTLPTLS{
					CA: &telemetryv1alpha1.ValueType{
						Value: serverCerts.CaCertPem.String(),
					},
				},
			),
		).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&pipeline,
		telemetrygen.NewPod(genNs, telemetrygen.SignalTypeTraces).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)
	resources = append(resources, oauth2server.K8sObjects()...)

	t.Cleanup(func() {
		// Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DeploymentReady(t, kitkyma.TraceGatewayName)
	assert.TracePipelineHealthy(t, pipelineName)
	assert.TracesFromNamespaceDelivered(t, backend, genNs)

	gatewayMetricsURL := suite.ProxyClient.ProxyURLForService(kitkyma.TraceGatewayMetricsService.Namespace, kitkyma.TraceGatewayMetricsService.Name, "metrics", ports.Metrics)
	assert.EmitsOTelCollectorMetrics(t, gatewayMetricsURL)
}
