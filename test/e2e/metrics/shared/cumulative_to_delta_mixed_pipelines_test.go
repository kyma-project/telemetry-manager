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

// TestCumulativeToDeltaMixedPipelines verifies that when two pipelines consume the same metrics,
// the delta pipeline receives Delta temporality while the cumulative pipeline receives Cumulative.
func TestCumulativeToDeltaMixedPipelines(t *testing.T) {
	tests := []struct {
		name             string
		labels           []string
		inputBuilder     func(includeNs string) telemetryv1beta1.MetricPipelineInput
		generatorBuilder func(ns string) []client.Object
		expectAgent      bool
	}{
		{
			name:   "agent",
			labels: []string{suite.LabelMetricAgent},
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
					telemetrygen.NewPod(ns, telemetrygen.SignalTypeMetrics, telemetrygen.WithMetricType("Sum")).K8sObject(),
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			suite.SetupTest(t, tc.labels...)

			var (
				uniquePrefix  = unique.Prefix(tc.name)
				pipelineDelta = uniquePrefix("delta")
				pipelineCumul = uniquePrefix("cumul")
				backendNs     = uniquePrefix("backend")
				genNs         = uniquePrefix("gen")
			)

			backendDelta := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName("backend-delta"))
			backendCumul := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName("backend-cumul"))

			pipelineWithDelta := testutils.NewMetricPipelineBuilder().
				WithName(pipelineDelta).
				WithInput(tc.inputBuilder(genNs)).
				WithTemporality(telemetryv1beta1.TemporalityDelta).
				WithMetricPipelineOTLPOutput(testutils.OTLPEndpoint(backendDelta.EndpointHTTP())).
				Build()

			pipelineWithCumulative := testutils.NewMetricPipelineBuilder().
				WithName(pipelineCumul).
				WithInput(tc.inputBuilder(genNs)).
				WithMetricPipelineOTLPOutput(testutils.OTLPEndpoint(backendCumul.EndpointHTTP())).
				Build()

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				new(pipelineWithDelta),
				new(pipelineWithCumulative),
			}
			resources = append(resources, tc.generatorBuilder(genNs)...)
			resources = append(resources, backendDelta.K8sObjects()...)
			resources = append(resources, backendCumul.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backendDelta)
			assert.BackendReachable(t, backendCumul)
			assert.DaemonSetReady(t, kitkyma.OTLPGatewayName)

			if tc.expectAgent {
				assert.DaemonSetReady(t, kitkyma.MetricAgentName)
			}

			assert.MetricPipelineHealthy(t, pipelineDelta)
			assert.MetricPipelineHealthy(t, pipelineCumul)

			// Delta pipeline receives Sum metrics with Delta temporality
			assert.BackendDataEventuallyMatches(t, backendDelta,
				metricmatchers.HaveFlatMetrics(ContainElement(SatisfyAll(
					metricmatchers.HaveType(Equal("Sum")),
					metricmatchers.HaveAggregationTemporality(Equal("Delta")),
					metricmatchers.HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", genNs)),
				))),
			)

			// Cumulative pipeline receives Sum metrics with Cumulative temporality (no conversion)
			assert.BackendDataEventuallyMatches(t, backendCumul,
				metricmatchers.HaveFlatMetrics(ContainElement(SatisfyAll(
					metricmatchers.HaveType(Equal("Sum")),
					metricmatchers.HaveAggregationTemporality(Equal("Cumulative")),
					metricmatchers.HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", genNs)),
				))),
			)
		})
	}
}
