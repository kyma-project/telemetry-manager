package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestTransform(t *testing.T) {
	tests := []struct {
		label            string
		name             string
		inputBuilder     func(includeNs string) telemetryv1beta1.MetricPipelineInput
		generatorBuilder func(ns string) []client.Object
		transformSpec    telemetryv1beta1.TransformSpec
		assertion        types.GomegaMatcher
		expectAgent      bool
	}{
		{
			label: suite.LabelMetricAgentSetC,
			name:  "with-where",
			inputBuilder: func(includeNs string) telemetryv1beta1.MetricPipelineInput {
				return testutils.BuildMetricPipelineRuntimeInput(testutils.IncludeNamespaces(includeNs))
			},
			generatorBuilder: func(ns string) []client.Object {
				generator := prommetricgen.New(ns)

				return []client.Object{
					generator.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				}
			},
			transformSpec: telemetryv1beta1.TransformSpec{
				Statements: []string{"set(datapoint.attributes[\"system\"], \"false\") where not IsMatch(resource.attributes[\"k8s.namespace.name\"], \".*-system\")"},
			},
			assertion: metric.HaveFlatMetrics(ContainElement(SatisfyAll(
				metric.HaveResourceAttributes(Not(HaveKeyWithValue("k8s.namespace.name", "kyma-system"))),
				metric.HaveMetricAttributes(HaveKeyWithValue("system", "false")),
			))),
			expectAgent: true,
		},
		{
			label: suite.LabelMetricAgentSetC,
			name:  "cond-and-stmts",
			inputBuilder: func(includeNs string) telemetryv1beta1.MetricPipelineInput {
				return testutils.BuildMetricPipelineRuntimeInput(testutils.IncludeNamespaces(includeNs))
			},
			generatorBuilder: func(ns string) []client.Object {
				generator := prommetricgen.New(ns)

				return []client.Object{
					generator.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				}
			},
			transformSpec: telemetryv1beta1.TransformSpec{
				Conditions: []string{"metric.type != \"\""},
				Statements: []string{"set(metric.description, \"FooMetric\")"},
			},
			assertion: metric.HaveFlatMetrics(ContainElement(SatisfyAll(
				metric.HaveDescription(Equal("FooMetric")),
			))),
			expectAgent: true,
		},
		{
			label: suite.LabelMetricAgentSetC,
			name:  "infer-context",
			inputBuilder: func(includeNs string) telemetryv1beta1.MetricPipelineInput {
				return testutils.BuildMetricPipelineRuntimeInput(testutils.IncludeNamespaces(includeNs))
			},
			generatorBuilder: func(ns string) []client.Object {
				generator := prommetricgen.New(ns)

				return []client.Object{
					generator.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				}
			},
			transformSpec: telemetryv1beta1.TransformSpec{
				Statements: []string{"set(resource.attributes[\"test\"], \"passed\")",
					"set(metric.description, \"test passed\")",
				},
			},
			assertion: metric.HaveFlatMetrics(ContainElement(SatisfyAll(
				metric.HaveResourceAttributes(HaveKeyWithValue("test", "passed")),
				metric.HaveDescription(Equal("test passed")),
			))),
			expectAgent: true,
		},
		{
			label: suite.LabelMetricGatewaySetC,
			name:  "with-where",
			inputBuilder: func(includeNs string) telemetryv1beta1.MetricPipelineInput {
				return testutils.BuildMetricPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			generatorBuilder: func(ns string) []client.Object {
				return []client.Object{
					telemetrygen.NewPod(ns, telemetrygen.SignalTypeMetrics).K8sObject(),
				}
			},
			transformSpec: telemetryv1beta1.TransformSpec{
				Statements: []string{"set(datapoint.attributes[\"system\"], \"false\") where not IsMatch(resource.attributes[\"k8s.namespace.name\"], \".*-system\")"},
			},
			assertion: metric.HaveFlatMetrics(ContainElement(SatisfyAll(
				metric.HaveResourceAttributes(Not(HaveKeyWithValue("k8s.namespace.name", "kyma-system"))),
				metric.HaveMetricAttributes(HaveKeyWithValue("system", "false")),
			))),
		},
		{
			label: suite.LabelMetricGatewaySetC,
			name:  "cond-and-stmts",
			inputBuilder: func(includeNs string) telemetryv1beta1.MetricPipelineInput {
				return testutils.BuildMetricPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			generatorBuilder: func(ns string) []client.Object {
				return []client.Object{
					telemetrygen.NewPod(ns, telemetrygen.SignalTypeMetrics).K8sObject(),
				}
			},
			transformSpec: telemetryv1beta1.TransformSpec{
				Conditions: []string{"metric.type != \"\""},
				Statements: []string{"set(metric.description, \"FooMetric\")"},
			},
			assertion: metric.HaveFlatMetrics(ContainElement(SatisfyAll(
				metric.HaveDescription(Equal("FooMetric")),
			))),
		},
		{
			label: suite.LabelMetricGatewaySetC,
			name:  "infer-context",
			inputBuilder: func(includeNs string) telemetryv1beta1.MetricPipelineInput {
				return testutils.BuildMetricPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			generatorBuilder: func(ns string) []client.Object {
				return []client.Object{
					telemetrygen.NewPod(ns, telemetrygen.SignalTypeMetrics).K8sObject(),
				}
			},
			transformSpec: telemetryv1beta1.TransformSpec{
				Statements: []string{"set(resource.attributes[\"test\"], \"passed\")",
					"set(metric.description, \"test passed\")",
				},
			},
			assertion: metric.HaveFlatMetrics(ContainElement(SatisfyAll(
				metric.HaveResourceAttributes(HaveKeyWithValue("test", "passed")),
				metric.HaveDescription(Equal("test passed")),
			))),
		},
	}

	for _, tc := range tests {
		suite.RegisterTestCase(t, tc.label)

		t.Run(tc.label, func(t *testing.T) {
			var (
				uniquePrefix = unique.Prefix("metrics", tc.name)
				pipelineName = uniquePrefix()
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)
			pipeline := testutils.NewMetricPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.inputBuilder(genNs)).
				WithTransform(tc.transformSpec).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
				Build()

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				&pipeline,
			}
			resources = append(resources, tc.generatorBuilder(genNs)...)
			resources = append(resources, backend.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.MetricGatewayName)

			if tc.expectAgent {
				assert.DaemonSetReady(t, kitkyma.MetricAgentName)
			}

			assert.MetricPipelineHealthy(t, pipelineName)

			assert.BackendDataEventuallyMatches(t, backend, tc.assertion)
		})
	}
}
