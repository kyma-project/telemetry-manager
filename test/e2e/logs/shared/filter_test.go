package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestFilter_OTel(t *testing.T) {
	tests := []struct {
		name               string
		labels             []string
		inputBuilder       func(includeNs string) telemetryv1beta1.LogPipelineInput
		resourceName       types.NamespacedName
	}{
		{
			name:   suite.LabelLogAgent,
			labels: []string{suite.LabelLogAgent},
			inputBuilder: func(includeNs string) telemetryv1beta1.LogPipelineInput {
				return testutils.BuildLogPipelineRuntimeInput(testutils.IncludeNamespaces(includeNs))
			},
			resourceName:       kitkyma.LogAgentName,
		},
		{
			name:   suite.LabelLogGateway,
			labels: []string{suite.LabelLogGateway},
			inputBuilder: func(includeNs string) telemetryv1beta1.LogPipelineInput {
				return testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			resourceName:       kitkyma.TelemetryOTLPGatewayName,
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

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)

			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.inputBuilder(genNs)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
				WithTransform(telemetryv1beta1.TransformSpec{
					Statements: []string{"set(log.attributes[\"test\"], \"passed\")"},
				}).
				WithFilter(telemetryv1beta1.FilterSpec{
					Conditions: []string{"log.attributes[\"test\"] == \"passed\""},
				}).
				Build()

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				&pipeline,
			}
			resources = append(resources, backend.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.BackendReachable(t, backend)
			assert.DaemonSetReady(t, tc.resourceName)
			assert.OTelLogPipelineHealthy(t, pipelineName)

			assert.BackendDataConsistentlyMatches(t, backend, Not(HaveFlatLogs(ContainElement(
				HaveAttributes(HaveKeyWithValue("test", "passed")),
			))))
		})
	}
}
