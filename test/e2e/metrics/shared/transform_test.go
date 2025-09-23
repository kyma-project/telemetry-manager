package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
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
		inputBuilder     func(includeNs string) telemetryv1alpha1.MetricPipelineInput
		generatorBuilder func(ns string) []client.Object
		transformSpec    telemetryv1alpha1.TransformSpec
		assertion        types.GomegaMatcher
		expectAgent      bool
	}{
		{
			label: suite.LabelMetricAgent,
			name:  "with-where",
			inputBuilder: func(includeNs string) telemetryv1alpha1.MetricPipelineInput {
				return testutils.BuildMetricPipelineRuntimeInput(testutils.IncludeNamespaces(includeNs))
			},
			generatorBuilder: func(ns string) []client.Object {
				generator := prommetricgen.New(ns)
				return []client.Object{
					generator.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				}
			},
			transformSpec: telemetryv1alpha1.TransformSpec{
				Statements: []string{"set(datapoint.attributes[\"system\"], \"false\") where not IsMatch(resource.attributes[\"k8s.namespace.name\"], \".*-system\")"},
			},
			assertion: metric.HaveFlatMetrics(ContainElement(SatisfyAll(
				metric.HaveResourceAttributes(Not(HaveKeyWithValue("k8s.namespace.name", "kyma-system"))),
				metric.HaveMetricAttributes(HaveKeyWithValue("system", "false")),
			))),
			expectAgent: true,
		},
		{
			label: suite.LabelMetricAgent,
			name:  "cond-and-stmts",
			inputBuilder: func(includeNs string) telemetryv1alpha1.MetricPipelineInput {
				return testutils.BuildMetricPipelineRuntimeInput(testutils.IncludeNamespaces(includeNs))
			},
			generatorBuilder: func(ns string) []client.Object {
				generator := prommetricgen.New(ns)
				return []client.Object{
					generator.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				}
			},
			transformSpec: telemetryv1alpha1.TransformSpec{
				Conditions: []string{"metric.type != \"\""},
				Statements: []string{"set(metric.description, \"FooMetric\")"},
			},
			assertion: metric.HaveFlatMetrics(ContainElement(SatisfyAll(
				metric.HaveDescription(Equal("FooMetric")),
			))),
			expectAgent: true,
		},
		{
			label: suite.LabelMetricAgent,
			name:  "infer-context",
			inputBuilder: func(includeNs string) telemetryv1alpha1.MetricPipelineInput {
				return testutils.BuildMetricPipelineRuntimeInput(testutils.IncludeNamespaces(includeNs))
			},
			generatorBuilder: func(ns string) []client.Object {
				generator := prommetricgen.New(ns)
				return []client.Object{
					generator.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
				}
			},
			transformSpec: telemetryv1alpha1.TransformSpec{
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
			label: suite.LabelMetricGateway,
			name:  "with-where",
			inputBuilder: func(includeNs string) telemetryv1alpha1.MetricPipelineInput {
				return testutils.BuildMetricPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			generatorBuilder: func(ns string) []client.Object {
				return []client.Object{
					telemetrygen.NewPod(ns, telemetrygen.SignalTypeMetrics).K8sObject(),
				}
			},
			transformSpec: telemetryv1alpha1.TransformSpec{
				Statements: []string{"set(datapoint.attributes[\"system\"], \"false\") where not IsMatch(resource.attributes[\"k8s.namespace.name\"], \".*-system\")"},
			},
			assertion: metric.HaveFlatMetrics(ContainElement(SatisfyAll(
				metric.HaveResourceAttributes(Not(HaveKeyWithValue("k8s.namespace.name", "kyma-system"))),
				metric.HaveMetricAttributes(HaveKeyWithValue("system", "false")),
			))),
		},
		{
			label: suite.LabelMetricGateway,
			name:  "cond-and-stmts",
			inputBuilder: func(includeNs string) telemetryv1alpha1.MetricPipelineInput {
				return testutils.BuildMetricPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			generatorBuilder: func(ns string) []client.Object {
				return []client.Object{
					telemetrygen.NewPod(ns, telemetrygen.SignalTypeMetrics).K8sObject(),
				}
			},
			transformSpec: telemetryv1alpha1.TransformSpec{
				Conditions: []string{"metric.type != \"\""},
				Statements: []string{"set(metric.description, \"FooMetric\")"},
			},
			assertion: metric.HaveFlatMetrics(ContainElement(SatisfyAll(
				metric.HaveDescription(Equal("FooMetric")),
			))),
		},
		{
			label: suite.LabelMetricGateway,
			name:  "infer-context",
			inputBuilder: func(includeNs string) telemetryv1alpha1.MetricPipelineInput {
				return testutils.BuildMetricPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			generatorBuilder: func(ns string) []client.Object {
				return []client.Object{
					telemetrygen.NewPod(ns, telemetrygen.SignalTypeMetrics).K8sObject(),
				}
			},
			transformSpec: telemetryv1alpha1.TransformSpec{
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
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, suite.LabelExperimental)

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
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
				Build()

			resources := []client.Object{
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(genNs).K8sObject(),
				&pipeline,
			}
			resources = append(resources, tc.generatorBuilder(genNs)...)
			resources = append(resources, backend.K8sObjects()...)

			t.Cleanup(func() {
				Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
			})
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
