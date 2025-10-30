package traces

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/trace"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestFilter(t *testing.T) {
	var (
		uniquePrefix = unique.Prefix(suite.LabelTraces)
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
		genNs        = uniquePrefix("gen")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces)
	pipeline := testutils.NewTracePipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		WithTransform(telemetryv1alpha1.TransformSpec{
			Statements: []string{"set(span.attributes[\"test\"], \"passed\")"},
		}).
		WithFilter(telemetryv1alpha1.FilterSpec{
			Conditions: []string{"span.attributes[\"test\"] == \"passed\""},
		}).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&pipeline,
		telemetrygen.NewPod(genNs, telemetrygen.SignalTypeTraces).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		// Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DeploymentReady(t, kitkyma.TraceGatewayName)
	assert.TracePipelineHealthy(t, pipelineName)

	assert.BackendDataConsistentlyMatches(t, backend, Not(HaveFlatTraces(ContainElement(
		HaveSpanAttributes(HaveKeyWithValue("test", "passed")),
	))))
}
