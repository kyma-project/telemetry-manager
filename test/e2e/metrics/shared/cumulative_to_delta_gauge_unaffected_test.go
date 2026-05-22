package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	metricmatchers "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

// TestCumulativeToDeltaGaugeUnaffected verifies that Gauge metrics are not affected
// by the cumulativetodelta processor (Gauges have no aggregation temporality).
func TestCumulativeToDeltaGaugeUnaffected(t *testing.T) {
	tests := []struct {
		name             string
		labels           []string
		inputBuilder     func(includeNs string) telemetryv1beta1.MetricPipelineInput
		generatorBuilder func(ns string) []client.Object
		metricNames      []string
		expectAgent      bool
	}{
		{
			name:   "agent",
			labels: []string{suite.LabelMetricAgent},
			inputBuilder: func(includeNs string) telemetryv1beta1.MetricPipelineInput {
				return testutils.BuildMetricPipelineAgentInput(false, true, false, testutils.IncludeNamespaces(includeNs))
			},
			generatorBuilder: func(ns string) []client.Object {
				generator := prommetricgen.New(ns)

				return []client.Object{
					generator.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
					generator.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				}
			},
			metricNames: []string{prommetricgen.MetricCPUTemperature.Name},
			expectAgent: true,
		},
		{
			name:   "gateway",
			labels: []string{suite.LabelMetricGateway},
			inputBuilder: func(includeNs string) telemetryv1beta1.MetricPipelineInput {
				return testutils.BuildMetricPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			generatorBuilder: func(ns string) []client.Object {
				return []client.Object{
					telemetrygen.NewPod(ns, telemetrygen.SignalTypeMetrics).K8sObject(),
				}
			},
			metricNames: telemetrygen.MetricNames,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			suite.SetupTest(t, tc.labels...)

			var (
				uniquePrefix = unique.Prefix(tc.name)
				pipelineName = uniquePrefix()
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)
			pipeline := testutils.NewMetricPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.inputBuilder(genNs)).
				WithTemporality(telemetryv1beta1.TemporalityDelta).
				WithMetricPipelineOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
				Build()

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				new(pipeline),
			}
			resources = append(resources, tc.generatorBuilder(genNs)...)
			resources = append(resources, backend.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend)
			assert.DaemonSetReady(t, kitkyma.OTLPGatewayName)

			if tc.expectAgent {
				assert.DaemonSetReady(t, kitkyma.MetricAgentName)
			}

			assert.MetricPipelineHealthy(t, pipelineName)

			// Gauge metrics should still be delivered with no aggregation temporality
			assert.BackendDataEventuallyMatches(t, backend,
				metricmatchers.HaveFlatMetrics(ContainElement(SatisfyAll(
					metricmatchers.HaveName(BeElementOf(tc.metricNames)),
					metricmatchers.HaveType(Equal("Gauge")),
					metricmatchers.HaveAggregationTemporality(BeEmpty()),
					metricmatchers.HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", genNs)),
				))),
			)
		})
	}
}
