package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/metrics/runtime"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMultiPipelineBroken(t *testing.T) {
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
			label: suite.LabelMetricGatewaySetB,
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
			suite.RegisterTestCase(t, tc.label)

			var (
				uniquePrefix        = unique.Prefix(tc.label)
				healthyPipelineName = uniquePrefix("healthy")
				brokenPipelineName  = uniquePrefix("broken")
				backendNs           = uniquePrefix("backend")
				genNs               = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)

			healthyPipeline := testutils.NewMetricPipelineBuilder().
				WithName(healthyPipelineName).
				WithInput(tc.inputBuilder(genNs)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
				Build()

			brokenPipeline := testutils.NewMetricPipelineBuilder().
				WithName(brokenPipelineName).
				WithInput(tc.inputBuilder(genNs)).
				WithOTLPOutput(testutils.OTLPEndpointFromSecret("dummy", "dummy", "dummy")). // broken pipeline ref
				Build()

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				&healthyPipeline,
				&brokenPipeline,
			}
			resources = append(resources, tc.generatorBuilder(genNs)...)
			resources = append(resources, backend.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.MetricGatewayName)

			if suite.ExpectAgent(tc.label) {
				assert.DaemonSetReady(t, kitkyma.MetricAgentName)
			}

			assert.MetricPipelineHealthy(t, healthyPipelineName)

			assert.MetricPipelineHasCondition(t, brokenPipeline.Name, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonReferencedSecretMissing,
			})

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
