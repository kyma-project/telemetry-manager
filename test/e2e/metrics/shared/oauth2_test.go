package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/metrics/runtime"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/oauth2mock"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestOAuth2(t *testing.T) {
	tests := []struct {
		label            string
		inputBuilder     func(includeNs string) telemetryv1beta1.MetricPipelineInput
		generatorBuilder func(ns string) []client.Object
	}{
		{
			label: suite.LabelMetricAgentSetC,
			inputBuilder: func(includeNs string) telemetryv1beta1.MetricPipelineInput {
				return testutils.BuildMetricPipelineRuntimeInput(testutils.IncludeNamespaces(includeNs))
			},
			generatorBuilder: func(ns string) []client.Object {
				generator := prommetricgen.New(ns)

				return []client.Object{
					generator.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
					generator.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				}
			},
		},
		{
			label: suite.LabelMetricGatewaySetC,
			inputBuilder: func(includeNs string) telemetryv1beta1.MetricPipelineInput {
				return testutils.BuildMetricPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			generatorBuilder: func(ns string) []client.Object {
				return []client.Object{
					telemetrygen.NewPod(ns, telemetrygen.SignalTypeMetrics).K8sObject(),
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label, suite.LabelOAuth2)

			var (
				uniquePrefix = unique.Prefix(tc.label, suite.LabelOAuth2)
				pipelineName = uniquePrefix()
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			oauth2server := oauth2mock.New(backendNs)

			serverCerts, _, err := testutils.NewCertBuilder(kitbackend.DefaultName, backendNs).Build()
			Expect(err).ToNot(HaveOccurred())

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics,
				kitbackend.WithTLS(*serverCerts),
				kitbackend.WithOIDCAuth(oauth2server.IssuerURL(), oauth2server.Audience()),
			)

			pipeline := testutils.NewMetricPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.inputBuilder(genNs)).
				WithOTLPOutput(
					testutils.OTLPEndpoint(backend.EndpointHTTPS()),
					testutils.OTLPOAuth2(
						testutils.OAuth2ClientID("the-mock-does-not-verify"),
						testutils.OAuth2ClientSecret("the-mock-does-not-verify"),
						testutils.OAuth2TokenURL(oauth2server.TokenEndpoint()),
						testutils.OAuth2Params(map[string]string{"grant_type": "client_credentials"}),
					),
					testutils.OTLPClientTLSFromString(serverCerts.CaCertPem.String()),
				).
				Build()

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				&pipeline,
			}
			resources = append(resources, tc.generatorBuilder(genNs)...)
			resources = append(resources, oauth2server.K8sObjects()...)
			resources = append(resources, backend.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.DeploymentReady(t, oauth2server.NamespacedName())
			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.MetricGatewayName)

			if suite.ExpectAgent(tc.label) {
				assert.DaemonSetReady(t, kitkyma.MetricAgentName)
			}

			assert.MetricPipelineHealthy(t, pipelineName)

			if suite.ExpectAgent(tc.label) {
				assert.MetricsFromNamespaceDelivered(t, backend, genNs, runtime.DefaultMetricsNames)

				agentMetricsURL := suite.ProxyClient.ProxyURLForService(kitkyma.MetricAgentMetricsService.Namespace, kitkyma.MetricAgentMetricsService.Name, "metrics", ports.Metrics)
				assert.EmitsOTelCollectorMetrics(t, agentMetricsURL)
			} else {
				assert.MetricsFromNamespaceDelivered(t, backend, genNs, telemetrygen.MetricNames)

				gatewayMetricsURL := suite.ProxyClient.ProxyURLForService(kitkyma.MetricGatewayMetricsService.Namespace, kitkyma.MetricGatewayMetricsService.Name, "metrics", ports.Metrics)
				assert.EmitsOTelCollectorMetrics(t, gatewayMetricsURL)
			}
		})
	}
}
