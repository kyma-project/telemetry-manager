package metrics

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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestTransform(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelExperimental)

	tt := []struct {
		name          string
		transformSpec telemetryv1alpha1.TransformSpec
		assertion     types.GomegaMatcher
	}{
		{
			name: "with-where",
			transformSpec: telemetryv1alpha1.TransformSpec{
				Statements: []string{"set(datapoint.attributes[\"system\"], \"false\") where not IsMatch(resource.attributes[\"k8s.namespace.name\"], \".*-system\")"},
			},
			assertion: metric.HaveFlatMetrics(ContainElement(SatisfyAll(
				metric.HaveResourceAttributes(Not(HaveKeyWithValue("k8s.namespace.name", "kyma-system"))),
				metric.HaveMetricAttributes(HaveKeyWithValue("system", "false")),
			))),
		}, {
			name: "cond-and-stmts",
			transformSpec: telemetryv1alpha1.TransformSpec{
				Conditions: []string{"metric.type != \"\""},
				Statements: []string{"set(metric.description, \"FooMetric\")"},
			},
			assertion: metric.HaveFlatMetrics(ContainElement(SatisfyAll(
				metric.HaveDescription(Equal("FooMetric")),
			))),
		}, {
			name: "infer-context",
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

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var (
				uniquePrefix = unique.Prefix(tc.name)
				pipelineName = uniquePrefix()
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)
			pipeline := testutils.NewMetricPipelineBuilder().
				WithName(pipelineName).
				WithTransform(tc.transformSpec).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
				Build()

			resources := []client.Object{
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(genNs).K8sObject(),
				&pipeline,
				telemetrygen.NewPod(genNs, telemetrygen.SignalTypeMetrics).K8sObject(),
			}
			resources = append(resources, backend.K8sObjects()...)

			t.Cleanup(func() {
				Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
			})
			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.MetricGatewayName)
			assert.MetricPipelineHealthy(t, pipelineName)

			assert.BackendDataEventuallyMatches(t, backend, tc.assertion)
		})
	}
}
