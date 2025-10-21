package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestFilter(t *testing.T) {
	tests := []struct {
		label        string
		inputBuilder func(includeNs string) telemetryv1alpha1.MetricPipelineInput
	}{
		{
			label: suite.LabelMetricAgentSetC,
			inputBuilder: func(includeNs string) telemetryv1alpha1.MetricPipelineInput {
				return testutils.BuildMetricPipelineRuntimeInput(testutils.IncludeNamespaces(includeNs))
			},
		},
		{
			label: suite.LabelMetricGatewaySetC,
			inputBuilder: func(includeNs string) telemetryv1alpha1.MetricPipelineInput {
				return testutils.BuildMetricPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, suite.LabelExperimental)

			var (
				uniquePrefix = unique.Prefix(tc.label)
				pipelineName = uniquePrefix()
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)
			pipeline := testutils.NewMetricPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.inputBuilder(genNs)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
				WithTransform(telemetryv1alpha1.TransformSpec{
					Statements: []string{"set(datapoint.attributes[\"test\"], \"passed\")"},
				}).
				WithFilter(telemetryv1alpha1.FilterSpec{
					Conditions: []string{"datapoint.attributes[\"test\"] == \"passed\""},
				}).
				Build()

			resources := []client.Object{
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(genNs).K8sObject(),
				&pipeline,
			}
			resources = append(resources, backend.K8sObjects()...)

			t.Cleanup(func() {
				Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
			})
			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.MetricGatewayName)

			if suite.ExpectAgent(tc.label) {
				assert.DaemonSetReady(t, kitkyma.MetricAgentName)
			}

			assert.MetricPipelineHealthy(t, pipelineName)

			assert.BackendDataConsistentlyMatches(t, backend, Not(HaveFlatMetrics(ContainElement(
				HaveMetricAttributes(HaveKeyWithValue("test", "passed")),
			))))
		})
	}
}
