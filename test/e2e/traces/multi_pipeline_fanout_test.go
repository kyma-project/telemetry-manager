package traces

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMultiPipelineFanout(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTraces)

	var (
		uniquePrefix  = unique.Prefix()
		backendNs     = uniquePrefix("backend")
		genNs         = uniquePrefix("gen")
		pipeline1Name = uniquePrefix("1")
		pipeline2Name = uniquePrefix("2")
	)

	backend1 := kitbackend.New(backendNs, kitbackend.SignalTypeTraces, kitbackend.WithName("backend1"))
	backend2 := kitbackend.New(backendNs, kitbackend.SignalTypeTraces, kitbackend.WithName("backend2"))

	pipeline1 := testutils.NewTracePipelineBuilder().
		WithName(pipeline1Name).
		WithOTLPOutput(testutils.OTLPEndpoint(backend1.EndpointHTTP())).
		Build()

	pipeline2 := testutils.NewTracePipelineBuilder().
		WithName(pipeline2Name).
		WithOTLPOutput(testutils.OTLPEndpoint(backend2.EndpointHTTP())).
		Build()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(genNs).K8sObject(),
		&pipeline1,
		&pipeline2,
		telemetrygen.NewPod(genNs, telemetrygen.SignalTypeTraces).K8sObject(),
	}
	resources = append(resources, backend1.K8sObjects()...)
	resources = append(resources, backend2.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend1)
	assert.BackendReachable(t, backend2)
	assert.DeploymentReady(t, kitkyma.TraceGatewayName)
	assert.TracePipelineHealthy(t, pipeline1Name)
	assert.TracePipelineHealthy(t, pipeline2Name)

	assert.TracesFromNamespaceDelivered(t, backend1, genNs)
	assert.TracesFromNamespaceDelivered(t, backend2, genNs)
}
